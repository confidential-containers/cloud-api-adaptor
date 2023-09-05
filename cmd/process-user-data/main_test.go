package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
)

// Test server to simulate the metadata service
func startTestServer() *httptest.Server {

	// Create a handler function for the desired path /metadata/instance/compute/userData
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Check that the request path is correct
		if r.URL.Path != "/metadata/instance/compute/userData" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		// Check that the request method is correct
		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}

		// Create base64 encoded test data
		testUserData := base64.StdEncoding.EncodeToString([]byte("test data"))

		// Write the test data to the response
		if _, err := io.WriteString(w, testUserData); err != nil {
			http.Error(w, "Error writing response.", http.StatusNotFound)
		}
	}

	// Create a test server
	srv := httptest.NewServer(http.HandlerFunc(handler))

	fmt.Printf("Started metadata server at srv.URL: %s\n", srv.URL)

	return srv

}

// test server, serving plain text userData
func startTestServerPlainText() *httptest.Server {

	// Create a handler function for the desired path /metadata/instance/compute/userData
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Check that the request path is correct
		if r.URL.Path != "/metadata/instance/compute/userData" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		// Check that the request method is correct
		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}

		// Write the test data to the response
		if _, err := io.WriteString(w, "test data"); err != nil {
			http.Error(w, "Error writing response.", http.StatusNotFound)
		}
	}

	// Create a test server
	srv := httptest.NewServer(http.HandlerFunc(handler))

	fmt.Printf("Started metadata server at srv.URL: %s\n", srv.URL)

	return srv

}

// TestGetUserData tests the getUserData function
func TestGetUserData(t *testing.T) {

	// Start a temporary HTTP server for the test simulating
	// the Azure metadata service
	srv := startTestServer()
	defer srv.Close()

	// Create a context
	ctx := context.Background()

	// Send request to srv.URL at path /metadata/instance/compute/userData

	reqPath := srv.URL + "/metadata/instance/compute/userData"
	// Call the getUserData function
	userData, _ := getUserData(ctx, reqPath)

	// Check that the userData is not empty
	if userData == "" {
		t.Fatalf("getUserData returned empty userData")
	}

	// Create a temporary file for the test
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the userData to the file
	if _, err := io.WriteString(tmpFile, userData); err != nil {
		t.Fatalf("failed to write userData to file: %v", err)
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	// Call the parseAndCopyUserData function with the userData and file path
	if err := parseAndCopyUserData(userData, tmpFile.Name()); err != nil {
		t.Fatalf("parseAndCopyUserData failed: %v", err)
	}

	// Read the file and check that the contents match the userData
	fileData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(fileData) != userData {
		t.Fatalf("file contents do not match userData: expected %q, got %q", userData, string(fileData))
	}
}

// TestInvalidGetUserDataInvalidUrl tests the getUserData function with an invalid URL
func TestInvalidGetUserDataInvalidUrl(t *testing.T) {

	// Create a context
	ctx := context.Background()

	// Send request to invalid URL
	reqPath := "invalidURL"
	// Call the getUserData function
	userData, _ := getUserData(ctx, reqPath)

	// Check that the userData is empty
	if userData != "" {
		t.Fatalf("getUserData returned non-empty userData")
	}
}

// TestInvalidGetUserDataEmptyUrl tests the getUserData function with an empty URL
func TestInvalidGetUserDataEmptyUrl(t *testing.T) {

	// Create a context
	ctx := context.Background()

	// Send request to empty URL
	reqPath := ""
	// Call the getUserData function
	userData, _ := getUserData(ctx, reqPath)

	// Check that the userData is empty
	if userData != "" {
		t.Fatalf("getUserData returned non-empty userData")
	}
}

func TestParseUserData(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary file for the test
	tmpFile, err := os.CreateTemp(tmpDir, "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testData := "test data"

	// Call the parseAndCopyUserData function with the test data and file path
	if err := parseAndCopyUserData(testData, tmpFile.Name()); err != nil {
		t.Fatalf("parseAndCopyUserData failed: %v", err)
	}

	// Read the file and check that the written contents by parseAndCopyUserData
	// match the test data
	fileData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(fileData) != testData {
		t.Fatalf("file contents do not match test data: expected %q, got %q", testData, string(fileData))
	}
}

// TestParseUserDataNonExistentFile tests the parseAndCopyUserData function with a non-existent file
func TestParseUserDataNonExistentFile(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := tmpDir + "/daemon.json"
	tmpFile1 := tmpDir + "/dir/daemon.json"
	testData := "test data"

	// Run a loop to call parseAndCopyUserData with different files
	for _, file := range []string{tmpFile, tmpFile1} {

		// Call the parseAndCopyUserData function with the test data and file path
		if err := parseAndCopyUserData(testData, file); err != nil {
			t.Fatalf("parseAndCopyUserData failed: %v", err)
		}

		// Read the file and check that the written contents by parseAndCopyUserData
		// match the test data
		fileData, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(fileData) != testData {
			t.Fatalf("file contents do not match test data: expected %q, got %q", testData, string(fileData))
		}
	}

}

// TestParsePlainTextUserData tests the parseAndCopyUserData function with plain text userData
func TestParsePlainTextUserData(t *testing.T) {

	// startTestServerPlainText
	srv := startTestServerPlainText()
	defer srv.Close()

	// Create a context
	ctx := context.Background()

	// Send request to srv.URL at path /metadata/instance/compute/userData

	reqPath := srv.URL + "/metadata/instance/compute/userData"
	// Call the getUserData function
	userData, _ := getUserData(ctx, reqPath)

	// Check that the userData is empty. Since plain text userData is not supported
	if userData != "" {
		t.Fatalf("getUserData returned userData")
	}

}

func TestGetConfigFromUserData(t *testing.T) {
	// Create a sample test data string
	testUserData := `{
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
	}`

	// Get the config from the test data
	config := getConfigFromUserData(testUserData)
	// Error if config struct is empty struct, not nil

	if config == (daemon.Config{}) {
		t.Fatalf("getConfigFromUserData failed")
	}

	// Verify that the config fields match the test data
	if config.PodNamespace != "default" {
		t.Fatalf("config.PodNamespace does not match test data: expected %q, got %q", "default", config.PodNamespace)
	}

	if config.PodName != "nginx-866fdb5bfb-b98nw" {
		t.Fatalf("config.PodName does not match test data: expected %q, got %q", "nginx-866fdb5bfb-b98nw", config.PodName)
	}

	if config.AAKBCParams != "cc_kbc::http://192.168.100.2:8080" {
		t.Fatalf("config.AAKBCParams does not match test data: expected %q, got %q", "cc_kbc::http://192.168.100.2:8080", config.AAKBCParams)
	}

}

func TestUpdateAAKBCParams(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary file for the test
	tmpFile, err := os.CreateTemp(tmpDir, "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	defer os.Remove(tmpFile.Name())

	// Write a sample agent config data to the file
	testAgentConfigData := `
		# This disables signature verification which now defaults to true.
		# We should consider a better solution. See #331 for more info
		enable_signature_verification=false

		# When using the agent-config.toml the KATA_AGENT_SERVER_ADDR env var seems to be ignored, so set it here
		server_addr="unix:///run/kata-containers/agent.sock"

		# This field sets up the KBC that attestation agent uses
		# This is replaced in the makefile steps so do not set it manually
		aa_kbc_params = "offline_fs_kbc::null"

		# temp workaround for kata-containers/kata-containers#5590
		[endpoints]
		allowed = [
		"AddARPNeighborsRequest",
		]`
	if _, err := tmpFile.WriteString(testAgentConfigData); err != nil {
		t.Fatalf("failed to write test data to file: %v", err)
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	testAAKBCParams := "cc_kbc::http://192.168.100.2:8080"
	// Call the updateAAKBCParams function with the test data and file path
	if err := updateAAKBCParams(testAAKBCParams, tmpFile.Name()); err != nil {
		t.Fatalf("updateAAKBCParams failed: %v", err)
	}

	// Read the file and check that the aa_kbc_params line has been replaced with the test data
	fileData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	expectedData := "aa_kbc_params = \"cc_kbc::http://192.168.100.2:8080\"\n"
	if !strings.Contains(string(fileData), expectedData) {
		t.Fatalf("file contents do not match expected data: expected %q, got %q", expectedData, string(fileData))
	}
}

func TestGetConfigFromLocalFile(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary file for the test
	tmpFile, err := os.CreateTemp(tmpDir, "test-config.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	defer os.Remove(tmpFile.Name())

	// Write some test data to the file
	testData := `{
		"aa-kbc-params": "test"
	}`

	if _, err := tmpFile.Write([]byte(testData)); err != nil {
		t.Fatal(err)
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	// Call the getConfigFromLocalFile function
	config := getConfigFromLocalFile(tmpFile.Name())

	fmt.Printf("%v\n", config)
	// Check if the config has been unmarshalled correctly
	expectedConfig := daemon.Config{
		AAKBCParams: "test",
	}

	if config != expectedConfig {
		t.Fatalf("Expected %+v, but got %+v", expectedConfig, config)
	}
}
