package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/aws"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/azure"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/spf13/cobra"
)

// Get the provider and the URL to retrieve the userData from the instance metadata service
func getProviderAndUserDataURL(ctx context.Context) (provider string, userDataUrl string) {

	if azure.IsAzure(ctx) {
		provider = providerAzure
		// If the VM is running on Azure, retrieve the userData from the Azure IMDS endpoint
		userDataUrl = azure.AzureUserDataImdsUrl
	}

	if aws.IsAWS(ctx) {
		provider = providerAws
		// If the VM is running on AWS, retrieve the userData from the AWS IMDS endpoint
		userDataUrl = aws.AWSUserDataImdsUrl
	}

	return provider, userDataUrl
}

// Add method to parse the userData and copy it to a file
func parseAndCopyUserData(userData string, dstFilePath string) error {

	// Write userData to file specified in the dstFilePath var
	// Create the directory and the file. Default is /peerpod/daemon.json

	// Split the dstFilePath into directory and file name
	splitPath := strings.Split(dstFilePath, "/")
	dir := strings.Join(splitPath[:len(splitPath)-1], "/")

	// Create the directory.
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %s", err)

	}

	// Create the file
	file, err := os.Create(dstFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err)
	}
	defer file.Close()

	// Write userData to file
	_, err = file.WriteString(userData)
	if err != nil {
		return fmt.Errorf("failed to write userData to file: %s", err)
	}

	fmt.Printf("Wrote userData to file: %s\n", dstFilePath)

	return nil

}

// Get daemon.Config from userData
func getConfigFromUserData(userData string) daemon.Config {

	// UnMarshal the userData into forwarder (daemon) Config struct
	var daemonConfig daemon.Config

	err := json.Unmarshal([]byte(userData), &daemonConfig)
	if err != nil {
		fmt.Printf("failed to unmarshal userData: %s\n", err)
		return daemon.Config{}
	}

	return daemonConfig
}

func provisionFiles(cmd *cobra.Command, args []string) error {

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	// Use retry.Do to retry the getUserData function until it succeeds
	// This is needed because the VM's userData is not available immediately
	// Have an option to either wait forever or timeout after a certain amount of time
	// https://github.com/avast/retry-go

	// Create context with the specified timeout
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(cfg.userDataFetchTimeout)*time.Second)
	defer cancel()

	// Get the provider and userData URL
	provider, userDataUrl := getProviderAndUserDataURL(ctx)

	fmt.Printf("provider: %s, userDataUrl: %s\n", provider, userDataUrl)

	err := retry.Do(
		func() error {

			var err error
			// Get the userData depending on the provider
			switch provider {
			case providerAzure:
				cfg.userData, err = azure.GetUserData(ctx, userDataUrl)
			case providerAws:
				cfg.userData, err = aws.GetUserData(ctx, userDataUrl)
			default:
				return fmt.Errorf("unsupported provider")
			}

			if err != nil {
				return fmt.Errorf("failed to get userData: %s", err)
			}

			if cfg.userData != "" && strings.Contains(cfg.userData, "podip") {
				return nil // Valid user data, stop retrying
			}
			return fmt.Errorf("invalid user data")
		},
		retry.Context(ctx),                // Use the context with timeout
		retry.Delay(5*time.Second),        // Set the delay between retries
		retry.LastErrorOnly(true),         // Only consider the last error for retry decision
		retry.DelayType(retry.FixedDelay), // Use fixed delay between retries
		retry.OnRetry(func(n uint, err error) { // Optional: log retry attempts
			fmt.Printf("Retry attempt %d: %v\n", n, err)
		}),
	)

	if err != nil {
		return fmt.Errorf("failed to get valid user data")
	}

	fmt.Printf("Valid user data: %s\n", cfg.userData)
	// Parse the userData and copy the specified values to the cfg.daemonConfigPath file
	if err := parseAndCopyUserData(cfg.userData, cfg.daemonConfigPath); err != nil {
		fmt.Printf("Error: Failed to parse userData: %s\n", err)
		return err
	}

	// Copy the authJson to the authJsonFilePath
	config := getConfigFromUserData(cfg.userData)
	if config.AuthJson != "" {
		// Create the file
		file, err := os.Create(defaultAuthJsonFilePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %s", err)
		}
		defer file.Close()

		// Write the authJson to the file
		_, err = file.WriteString(config.AuthJson)
		if err != nil {
			return fmt.Errorf("failed to write authJson to file: %s", err)
		}

	}

	return nil
}
