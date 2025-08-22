package userdata

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	retry "github.com/avast/retry-go/v4"
	yaml "gopkg.in/yaml.v2"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/initdata"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/paths"
)

const (
	ConfigParent = "/run/peerpod"
	DigestPath   = "/run/peerpod/initdata.digest"
	PolicyPath   = "/run/peerpod/policy.rego"
	// Ref: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
	AWSImdsURL         = "http://169.254.169.254/latest/dynamic/instance-identity/document"
	AWSUserDataImdsURL = "http://169.254.169.254/latest/user-data"
	// Ref: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service
	AzureImdsURL         = "http://169.254.169.254/metadata/instance/compute?api-version=2021-01-01"
	AzureUserDataImdsURL = "http://169.254.169.254/metadata/instance/compute/userData?api-version=2021-01-01&format=text"
	// Ref: https://cloud.google.com/compute/docs/storing-retrieving-metadata
	GcpImdsURL         = "http://metadata.google.internal/computeMetadata/v1/instance"
	GcpUserDataImdsURL = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/user-data"
	// Ref: https://www.alibabacloud.com/help/en/ecs/user-guide/customize-the-initialization-configuration-for-an-instance
	AlibabaCloudImdsURL         = "http://100.100.100.200/latest/dynamic/instance-identity/document"
	AlibabaCloudUserDataImdsURL = "http://100.100.100.200/latest/user-data"
)

var logger = log.New(log.Writer(), "[userdata/provision] ", log.LstdFlags|log.Lmsgprefix)
var WriteFilesList = []string{paths.AACfgPath, paths.CDHCfgPath, paths.ForwarderCfgPath, paths.AuthFilePath, paths.InitDataPath, paths.ScratchSpacePath}
var InitdDataFilesList = []string{paths.AACfgPath, paths.CDHCfgPath, PolicyPath}

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
		initdataPath:  paths.InitDataPath,
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
	url := AzureUserDataImdsURL
	logger.Printf("provider: Azure, userDataUrl: %s\n", url)
	return imdsGet(ctx, url, true, []kvPair{{"Metadata", "true"}})
}

type AWSUserDataProvider struct{ DefaultRetry }

func (a AWSUserDataProvider) GetUserData(ctx context.Context) ([]byte, error) {
	url := AWSUserDataImdsURL
	logger.Printf("provider: AWS, userDataUrl: %s\n", url)
	// aws user data is not base64 encoded
	return imdsGet(ctx, url, false, nil)
}

type GCPUserDataProvider struct{ DefaultRetry }

func (g GCPUserDataProvider) GetUserData(ctx context.Context) ([]byte, error) {
	url := GcpUserDataImdsURL
	logger.Printf("provider: GCP, userDataUrl: %s\n", url)
	return imdsGet(ctx, url, true, []kvPair{{"Metadata-Flavor", "Google"}})
}

type FileUserDataProvider struct{ DefaultRetry }

func (a FileUserDataProvider) GetUserData(ctx context.Context) ([]byte, error) {
	path := paths.UserDataPath
	logger.Printf("provider: File, userDataPath: %s\n", path)
	userData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %s", err)
	}

	return userData, nil
}

type AlibabaCloudDataProvider struct{ DefaultRetry }

func (a AlibabaCloudDataProvider) GetUserData(ctx context.Context) ([]byte, error) {
	url := AlibabaCloudUserDataImdsURL
	logger.Printf("provider: AlibabaCloud, userDataUrl: %s\n", url)
	return imdsGet(ctx, url, false, nil)
}

func newProvider(ctx context.Context) (UserDataProvider, error) {
	// This checks for the presence of a file and doesn't rely on http req like the
	// azure, aws ones, thereby making it faster and hence checking this first
	if hasUserDataFile() {
		return FileUserDataProvider{}, nil
	}

	if isAzureVM() {
		return AzureUserDataProvider{}, nil
	}

	if isAWSVM(ctx) {
		return AWSUserDataProvider{}, nil
	}

	if isGCPVM(ctx) {
		return GCPUserDataProvider{}, nil
	}

	if isAlibabaCloudVM() {
		return AlibabaCloudDataProvider{}, nil
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
		return fmt.Errorf("error stat initdata file: %w", err)
	}

	fileReader, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error read initdata file: %w", err)
	}

	id, err := initdata.Parse(fileReader)
	if err != nil {
		return fmt.Errorf("error parse initdata: %w", err)
	}

	for key, value := range id.Body.Data {
		path := filepath.Join(cfg.parentPath, key)
		if isAllowed(path, cfg.initdataFiles) {
			if err := writeFile(path, []byte(value)); err != nil {
				return fmt.Errorf("error write a file in initdata: %w", err)
			}
		} else {
			logger.Printf("File: %s is not allowed in initdata.\n", key)
		}
	}

	// the hash in digestPath will also be used by attester
	err = writeFile(cfg.digestPath, []byte(id.Digest))
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
