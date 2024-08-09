package userdata

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/aa"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/agent"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/cdh"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/aws"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/azure"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/docker"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v2"
)

const (
	ConfigParent = "/run/peerpod"
	DigestPath   = "/run/peerpod/initdata.digest"
	PolicyPath   = "/run/peerpod/policy.rego"
)

var logger = log.New(log.Writer(), "[userdata/provision] ", log.LstdFlags|log.Lmsgprefix)
var WriteFilesList = []string{aa.ConfigFilePath, cdh.ConfigFilePath, agent.ConfigFilePath, forwarder.DefaultConfigPath, cloud.AuthFilePath, cloud.InitdataPath}
var InitdDataFilesList = []string{aa.ConfigFilePath, cdh.ConfigFilePath, PolicyPath}

type Config struct {
	fetchTimeout  int
	digestPath    string
	initdataPath  string
	parentPath    string
	writeFiles    []string
	initdataFiles []string
}

func NewConfig(fetchTimeout int) *Config {
	return &Config{
		fetchTimeout:  fetchTimeout,
		parentPath:    ConfigParent,
		initdataPath:  cloud.InitdataPath,
		digestPath:    DigestPath,
		writeFiles:    WriteFilesList,
		initdataFiles: InitdDataFilesList,
	}
}

type WriteFile struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

type CloudConfig struct {
	WriteFiles []WriteFile `yaml:"write_files"`
}

type InitData struct {
	Algorithm string            `toml:"algorithm"`
	Version   string            `toml:"version"`
	Data      map[string]string `toml:"data,omitempty"`
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

func writeFile(path string, bytes []byte) error {
	// Ensure the parent directory exists
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	err = os.WriteFile(path, bytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	logger.Printf("Wrote %s\n", path)
	return nil
}

func isAllowed(path string, filesList []string) bool {
	for _, listedFile := range filesList {
		if listedFile == path {
			return true
		}
	}
	return false
}

func processCloudConfig(cfg *Config, cc *CloudConfig) error {
	for _, wf := range cc.WriteFiles {
		path := wf.Path
		bytes := []byte(wf.Content)
		if isAllowed(path, cfg.writeFiles) {
			if err := writeFile(path, bytes); err != nil {
				return fmt.Errorf("failed to write config file %s: %w", path, err)
			}
		} else {
			logger.Printf("File: %s is not allowed in WriteFiles.\n", path)
		}
	}

	return nil
}

func extractInitdataAndHash(cfg *Config) error {
	path := cfg.initdataPath
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Printf("File %s not found, skipped initdata processing.\n", path)
			return nil
		}
		return fmt.Errorf("Error stat initdata file: %w", err)
	}

	dataBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Error read initdata file: %w", err)
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(string(dataBytes))
	if err != nil {
		return fmt.Errorf("Error base64 decode initdata: %w", err)
	}
	initdata := InitData{}
	err = toml.Unmarshal(decodedBytes, &initdata)
	if err != nil {
		return fmt.Errorf("Error unmarshalling initdata: %w", err)
	}

	for key, value := range initdata.Data {
		path := filepath.Join(cfg.parentPath, key)
		if isAllowed(path, cfg.initdataFiles) {
			if err := writeFile(path, []byte(value)); err != nil {
				return fmt.Errorf("Error write a file in initdata: %w", err)
			}
		} else {
			logger.Printf("File: %s is not allowed in initdata.\n", key)
		}
	}

	checksumStr := ""
	switch initdata.Algorithm {
	case "sha256":
		hash := sha256.Sum256(decodedBytes)
		checksumStr = hex.EncodeToString(hash[:])
	case "sha384":
		hash := sha512.Sum384(decodedBytes)
		checksumStr = hex.EncodeToString(hash[:])
	case "sha512":
		hash := sha512.Sum512(decodedBytes)
		checksumStr = hex.EncodeToString(hash[:])
	default:
		return fmt.Errorf("Error creating initdata hash, the Algorithm %s not supported", initdata.Algorithm)
	}

	err = writeFile(cfg.digestPath, []byte(checksumStr)) // the hash in digestPath will also be used by attester
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", cfg.digestPath, err)
	}

	return nil
}

func ProvisionFiles(cfg *Config) error {
	bg := context.Background()
	duration := time.Duration(cfg.fetchTimeout) * time.Second
	ctx, cancel := context.WithTimeout(bg, duration)
	defer cancel()

	// some providers provision config files via process-user-data
	// some providers rely on cloud-init provision config files
	// all providers need extract files from initdata and calculate the hash value for attesters usage
	provider, _ := newProvider(ctx)
	if provider != nil {
		cc, err := retrieveCloudConfig(ctx, provider)
		if err != nil {
			return fmt.Errorf("failed to retrieve cloud config: %w", err)
		}

		if err = processCloudConfig(cfg, cc); err != nil {
			return fmt.Errorf("failed to process cloud config: %w", err)
		}
	} else {
		logger.Printf("unsupported user data provider, we extract and calculate initdata hash only.\n")
	}

	if err := extractInitdataAndHash(cfg); err != nil {
		return fmt.Errorf("failed to extract initdata hash: %w", err)
	}

	return nil
}
