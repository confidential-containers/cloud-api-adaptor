package azure

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

const (
	AzureImdsUrl         = "http://169.254.169.254/metadata/instance/compute?api-version=2021-01-01"
	AzureUserDataImdsUrl = "http://169.254.169.254/metadata/instance/compute/userData?api-version=2021-01-01&format=text"
)

// Method to check if the VM is running on Azure
// by checking if the Azure IMDS endpoint is reachable
// Set Metadata:true header to confirm that the VM is running on Azure
// If the VM is running on Azure, return true
func IsAzure(ctx context.Context) bool {

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
	// Example request for Azure.
	// curl -H Metadata:true --noproxy "*" "http://169.254.169.254/metadata/instance/compute/userData?api-version=2021-01-01&format=text" | base64 --decode

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", err)

	}
	// Add the required headers to the request
	req.Header.Add("Metadata", "true")

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
		    "auth-json": "..."

		}
	*/

	// The response is base64 encoded

	// Decode the base64 response
	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to decode b64 encoded userData: %s", err)
	}

	return decoded, nil
}
