package userdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/userdata/aws"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/userdata/azure"
	"gopkg.in/yaml.v2"
)

var logger = log.New(log.Writer(), "[userdata/provision] ", log.LstdFlags|log.Lmsgprefix)

type Config struct {
	authJsonPath     string
	daemonConfigPath string
	fetchTimeout     int
}

func NewConfig(authJsonPath, daemonConfigPath string, fetchTimeout int) *Config {
	return &Config{authJsonPath, daemonConfigPath, fetchTimeout}
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

func newProvider(ctx context.Context) (UserDataProvider, error) {
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

func parseDaemonConfig(content []byte) (*daemon.Config, error) {
	var dc daemon.Config
	err := json.Unmarshal(content, &dc)
	if err != nil {
		return nil, err
	}
	return &dc, nil
}

func findDaemonConfigEntry(path string, cc *CloudConfig) (*daemon.Config, []byte, error) {
	for _, wf := range cc.WriteFiles {
		if wf.Path != path {
			continue
		}
		bytes := []byte(wf.Content)
		daemonConfig, err := parseDaemonConfig(bytes)
		if err != nil {
			return nil, nil, err
		}
		return daemonConfig, bytes, nil
	}
	return nil, nil, fmt.Errorf("failed to find entry for %s in cloud config", path)
}

func writeFile(path string, bytes []byte) error {
	err := os.WriteFile(path, bytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	logger.Printf("Wrote %s\n", path)
	return nil
}

func processCloudConfig(cfg *Config, cc *CloudConfig) error {
	daemonConfig, bytes, err := findDaemonConfigEntry(cfg.daemonConfigPath, cc)
	if err != nil {
		return fmt.Errorf("failed to process daemon config: %w", err)
	}

	if err = writeFile(cfg.daemonConfigPath, bytes); err != nil {
		return fmt.Errorf("failed to write daemon config file: %w", err)
	}

	if daemonConfig.AuthJson != "" {
		bytes := []byte(daemonConfig.AuthJson)
		if err = writeFile(cfg.authJsonPath, bytes); err != nil {
			return fmt.Errorf("failed to write auth json file: %w", err)
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
