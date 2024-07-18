package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	GcpImdsUrl         = "http://metadata.google.internal/computeMetadata/v1/instance"
	GcpUserDataImdsUrl = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/user-data"
)

// Method to check if the VM is running on GCP
func IsGCP(ctx context.Context) bool {

	// Create a new HTTP client
	client := &http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GcpImdsUrl, nil)
	if err != nil {
		fmt.Printf("failed to create request: %s\n", err)
		return false
	}
	// Add the required headers to the request
	req.Header.Add("Metadata-Flavor", "Google")

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

// Method to retrieve userData from the instance metadata service
// and return it as a string
func GetUserData(ctx context.Context, url string) ([]byte, error) {

	// If url is empty then return empty string
	if url == "" {
		return nil, fmt.Errorf("url is empty")
	}

	// Create a new HTTP client
	client := &http.Client{}

	// Create a new request to retrieve the VM's userData
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", err)

	}
	// Add the required headers to the request
	req.Header.Add("Metadata-Flavor", "Google")

	// Send the request and retrieve the response
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %s", err)

	}
	defer resp.Body.Close()

	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve userData: %s", resp.Status)

	}

	// Read the response body and return it as a string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %s", err)

	}

	// Decode the base64 response
	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to decode b64 encoded userData: %s", err)
	}

	return decoded, nil
}
