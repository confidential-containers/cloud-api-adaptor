package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/aws"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/azure"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type WriteFile struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

type CloudConfig struct {
	WriteFiles []WriteFile `yaml:"write_files"`
}

func getProvider(ctx context.Context) (UserDataProvider, error) {
	if azure.IsAzure(ctx) {
		return AzureUserDataProvider{}, nil
	}

	if aws.IsAWS(ctx) {
		return AWSUserDataProvider{}, nil
	}

	return nil, fmt.Errorf("unsupported user data provider")
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

func getDaemonConfigContent(cc *CloudConfig) ([]byte, error) {
	for _, wf := range cc.WriteFiles {
		if wf.Path != cfg.daemonConfigPath {
			continue
		}
		return []byte(wf.Content), nil
	}
	return nil, fmt.Errorf("failed to find entry for %s in cloud config", cfg.daemonConfigPath)
}

func writeAuthJson(dc *daemon.Config) error {
	// Create the file
	file, err := os.Create(cfg.authJsonPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write the authJson to the file
	_, err = file.WriteString(dc.AuthJson)
	if err != nil {
		return fmt.Errorf("failed to write authJson to file: %w", err)
	}
	return nil
}

func writeDaemonConfig(content []byte) error {
	err := os.WriteFile(cfg.daemonConfigPath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write daemon config file: %w", err)
	}
	fmt.Printf("Wrote daemon config file: %s\n", cfg.daemonConfigPath)
	return nil
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
	fmt.Printf("provider: Azure, userDataUrl: %s\n", url)
	return azure.GetUserData(ctx, url)
}

type AWSUserDataProvider struct{ DefaultRetry }

func (a AWSUserDataProvider) GetUserData(ctx context.Context) ([]byte, error) {
	url := aws.AWSUserDataImdsUrl
	fmt.Printf("provider: AWS, userDataUrl: %s\n", url)
	return aws.GetUserData(ctx, url)
}

func getCloudConfig(ctx context.Context, provider UserDataProvider) (*CloudConfig, error) {
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
			fmt.Printf("Retry attempt %d: %v\n", n, err)
		}),
	)

	return &cc, err
}

func processCloudConfig(cc *CloudConfig) error {
	dcc, err := getDaemonConfigContent(cc)
	if err != nil {
		return err
	}

	dc, err := parseDaemonConfig(dcc)
	if err != nil {
		return err
	}

	if err = writeDaemonConfig(dcc); err != nil {
		return err
	}

	if dc.AuthJson != "" {
		if err := writeAuthJson(dc); err != nil {
			return err
		}
	}

	return nil
}

func provisionFiles(_ *cobra.Command, _ []string) error {
	bg := context.Background()
	duration := time.Duration(cfg.userDataFetchTimeout) * time.Second
	ctx, cancel := context.WithTimeout(bg, duration)
	defer cancel()

	provider, _ := getProvider(ctx)
	cc, err := getCloudConfig(ctx, provider)
	if err != nil {
		return fmt.Errorf("failed to get valid cloud config: %w", err)
	}

	if err = processCloudConfig(cc); err != nil {
		return fmt.Errorf("failed to process cloud config: %w", err)
	}
	return nil
}
