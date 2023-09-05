package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/spf13/cobra"
)

// Add a method to check if the VM is running on Azure
// by checking if the Azure IMDS endpoint is reachable
// Set Metadata:true header to confirm that the VM is running on Azure
// If the VM is running on Azure, return true
func isAzure(ctx context.Context) bool {

	// Create a new HTTP client
	client := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AzureImdsUrl, nil)
	if err != nil {
		fmt.Printf("failed to create request: %s\n", err)
		return false
	}
	// Add the required headers to the request
	req.Header.Add("Metadata", "true")

	// Send the request and retrieve the response
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("failed to send request: %s\n", err)
		return false
	}
	defer resp.Body.Close()

	// Check if the response was successful
	return resp.StatusCode == http.StatusOK
}

// Get the URL to retrieve the userData from the instance metadata service
func getUserDataURL(ctx context.Context) string {
	userDataUrl := ""

	if isAzure(ctx) {
		// If the VM is running on Azure, retrieve the userData from the Azure IMDS endpoint
		userDataUrl = AzureUserDataImdsUrl
	}

	return userDataUrl
}

// Add a method to retrieve userData from the instance metadata service
// and return it as a string
func getUserData(ctx context.Context, url string) (string, error) {

	// If url is empty then return empty string
	if url == "" {
		return "", fmt.Errorf("url is empty")
	}

	// Create a new HTTP client
	client := &http.Client{}

	// Create a new request to retrieve the VM's userData
	// Example request for Azure
	// curl -H Metadata:true --noproxy "*" "http://169.254.169.254/metadata/instance/compute/userData?api-version=2021-01-01&format=text" | base64 --decode

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %s", err)

	}
	// Add the required headers to the request
	req.Header.Add("Metadata", "true")

	// Send the request and retrieve the response
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %s", err)

	}
	defer resp.Body.Close()

	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to retrieve userData: %s", resp.Status)

	}

	// Read the response body and return it as a string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %s", err)

	}

	// Sample data
	/*
			{
		    "pod-network": {
		        "podip": "10.244.0.19/24",
		        "pod-hw-addr": "0e:8f:62:f3:81:ad",
		        "interface": "eth0",
		        "worker-node-ip": "10.224.0.4/16",
		        "tunnel-type": "vxlan",
		        "routes": [
		            {
		                "Dst": "",
		                "GW": "10.244.0.1",
		                "Dev": "eth0"
		            }
		        ],
		        "mtu": 1500,
		        "index": 1,
		        "vxlan-port": 8472,
		        "vxlan-id": 555001,
		        "dedicated": false
		    },
		    "pod-namespace": "default",
		    "pod-name": "nginx-866fdb5bfb-b98nw",
		    "tls-server-key": "-----BEGIN PRIVATE KEY-----\n....\n-----END PRIVATE KEY-----\n",
		    "tls-server-cert": "-----BEGIN CERTIFICATE-----\n....\n-----END CERTIFICATE-----\n",
		    "tls-client-ca": "-----BEGIN CERTIFICATE-----\n....\n-----END CERTIFICATE-----\n",
		    "aa-kbc-params": "cc_kbc::http://192.168.100.2:8080"

		}
	*/

	// The response is base64 encoded

	// Decode the base64 response
	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return "", fmt.Errorf("failed to decode b64 encoded userData: %s", err)
	}

	return string(decoded), nil
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

	// Get the userData URL
	userDataUrl := getUserDataURL(ctx)

	fmt.Printf("userDataUrl: %s\n", userDataUrl)

	err := retry.Do(
		func() error {
			cfg.userData, _ = getUserData(ctx, userDataUrl)
			if cfg.userData != "" && strings.Contains(cfg.userData, "podip") {
				return nil // Valid user data, stop retrying
			}
			return fmt.Errorf("invalid userdata")
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

	return nil
}
