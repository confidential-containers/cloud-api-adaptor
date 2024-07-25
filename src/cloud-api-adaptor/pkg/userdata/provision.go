package userdata

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/aws"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/azure"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/docker"
	"gopkg.in/yaml.v2"
)

var logger = log.New(log.Writer(), "[userdata/provision] ", log.LstdFlags|log.Lmsgprefix)

type paths struct {
	aaConfig     string
	agentConfig  string
	authJson     string
	cdhConfig    string
	daemonConfig string
}

type Config struct {
	fetchTimeout int
	paths        paths
}

func NewConfig(aaConfigPath, agentConfig, authJsonPath, daemonConfigPath, cdhConfig string, fetchTimeout int) *Config {
	ps := paths{
		aaConfig:     aaConfigPath,
		agentConfig:  agentConfig,
		authJson:     authJsonPath,
		cdhConfig:    cdhConfig,
		daemonConfig: daemonConfigPath,
	}
	return &Config{
		fetchTimeout: fetchTimeout,
		paths:        ps,
	}
}

type WriteFile struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

type CloudConfig struct {
	WriteFiles []WriteFile `yaml:"write_files"`
}

type UserDataProvider interface {
	GetUserData(ctx context.Context) ([]byte, error)
	GetRetryDelay() time.Duration
}

type DefaultRetry struct{}

func (d DefaultRetry) GetRetryDelay() time.Duration {
	return 5 * time.Second
}

type AzureUserDataProvider struct{ DefaultRetry }

func (a AzureUserDataProvider) GetUserData(ctx context.Context) ([]byte, error) {
	url := azure.AzureUserDataImdsUrl
	logger.Printf("provider: Azure, userDataUrl: %s\n", url)
	return azure.GetUserData(ctx, url)
}

type AWSUserDataProvider struct{ DefaultRetry }

func (a AWSUserDataProvider) GetUserData(ctx context.Context) ([]byte, error) {
	url := aws.AWSUserDataImdsUrl
	logger.Printf("provider: AWS, userDataUrl: %s\n", url)
	return aws.GetUserData(ctx, url)
}

type DockerUserDataProvider struct{ DefaultRetry }

func (a DockerUserDataProvider) GetUserData(ctx context.Context) ([]byte, error) {
	url := docker.DockerUserDataUrl
	logger.Printf("provider: Docker, userDataUrl: %s\n", url)
	return docker.GetUserData(ctx, url)
}

func newProvider(ctx context.Context) (UserDataProvider, error) {

	// This checks for the presence of a file and doesn't rely on http req like the
	// azure, aws ones, thereby making it faster and hence checking this first
	if docker.IsDocker(ctx) {
		return DockerUserDataProvider{}, nil
	}
	if azure.IsAzure(ctx) {
		return AzureUserDataProvider{}, nil
	}

	if aws.IsAWS(ctx) {
		return AWSUserDataProvider{}, nil
	}

	return nil, fmt.Errorf("unsupported user data provider")
}

func retrieveCloudConfig(ctx context.Context, provider UserDataProvider) (*CloudConfig, error) {
	var cc CloudConfig

	// Use retry.Do to retry the getUserData function until it succeeds
	// This is needed because the VM's userData is not available immediately
	err := retry.Do(
		func() error {
			ud, err := provider.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("failed to get user data: %w", err)
			}

			// We parse user data now, b/c we want to retry if it's not valid
			parsed, err := parseUserData(ud)
			if err != nil {
				return fmt.Errorf("failed to parse user data: %w", err)
			}
			cc = *parsed

			// Valid user data, stop retrying
			return nil
		},
		retry.Context(ctx),
		retry.Delay(provider.GetRetryDelay()),
		retry.LastErrorOnly(true),
		retry.DelayType(retry.FixedDelay),
		retry.OnRetry(func(n uint, err error) {
			logger.Printf("Retry attempt %d: %v\n", n, err)
		}),
	)

	return &cc, err
}

func parseUserData(userData []byte) (*CloudConfig, error) {
	var cc CloudConfig
	err := yaml.UnmarshalStrict(userData, &cc)
	if err != nil {
		return nil, err
	}
	return &cc, nil
}

func findConfigEntry(path string, cc *CloudConfig) []byte {
	for _, wf := range cc.WriteFiles {
		if wf.Path != path {
			continue
		}
		return []byte(wf.Content)
	}
	return nil
}

type entry struct {
	path     string
	optional bool
}

func (f *entry) writeFile(cc *CloudConfig) error {
	bytes := findConfigEntry(f.path, cc)
	if bytes == nil {
		if !f.optional {
			return fmt.Errorf("failed to find %s entry in cloud config", f.path)
		}
		return nil
	}

	// Ensure the parent directory exists
	err := os.MkdirAll(filepath.Dir(f.path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	err = os.WriteFile(f.path, bytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	logger.Printf("Wrote %s\n", f.path)
	return nil
}

func processCloudConfig(cfg *Config, cc *CloudConfig) error {
	entries := []entry{
		{path: cfg.paths.agentConfig, optional: false},
		{path: cfg.paths.daemonConfig, optional: false},
		{path: cfg.paths.aaConfig, optional: true},
		{path: cfg.paths.cdhConfig, optional: true},
		{path: cfg.paths.authJson, optional: true},
	}

	for _, e := range entries {
		err := e.writeFile(cc)
		if err != nil {
			return err
		}
	}

	return nil
}

func ProvisionFiles(cfg *Config) error {
	bg := context.Background()
	duration := time.Duration(cfg.fetchTimeout) * time.Second
	ctx, cancel := context.WithTimeout(bg, duration)
	defer cancel()

	provider, err := newProvider(ctx)
	if err != nil {
		return fmt.Errorf("failed to create UserData provider: %w", err)
	}

	cc, err := retrieveCloudConfig(ctx, provider)
	if err != nil {
		return fmt.Errorf("failed to retrieve cloud config: %w", err)
	}

	if err = processCloudConfig(cfg, cc); err != nil {
		return fmt.Errorf("failed to process cloud config: %w", err)
	}

	return nil
}
