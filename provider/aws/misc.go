package aws

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const (
	// Ref: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
	AWSImdsUrl         = "http://169.254.169.254/latest/dynamic/instance-identity/document"
	AWSUserDataImdsUrl = "http://169.254.169.254/latest/user-data"
)

// Method to check if the VM is running on AWS
// by checking if the AWS IMDS endpoint is reachable
// If the VM is running on AWS, return true
func IsAWS(ctx context.Context) bool {

	// Create a new HTTP client
	client := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AWSImdsUrl, nil)
	if err != nil {
		fmt.Printf("failed to create request: %s\n", err)
		return false
	}

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
	// Example request for AWS.
	// curl http://169.254.169.254/latest/user-data

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", err)

	}

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

	return body, nil
}
