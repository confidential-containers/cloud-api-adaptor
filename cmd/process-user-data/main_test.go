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
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/azure"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/stretchr/testify/assert"
)

var testDaemonConfig string = `{
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
	"aa-kbc-params": "cc_kbc::http://192.168.100.2:8080",
	"auth-json": "{\"auths\":{}}"
}`

// Test server to simulate the metadata service
func startTestServer() *httptest.Server {
	// Create base64 encoded test data
	testUserDataString := base64.StdEncoding.EncodeToString([]byte("test data"))

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
		if _, err := io.WriteString(w, testUserDataString); err != nil {
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
	userData, _ := azure.GetUserData(ctx, reqPath)

	// Check that the userData is not empty
	if userData == nil {
		t.Fatalf("getUserData returned empty userData")
	}
}

// TestInvalidGetUserDataInvalidUrl tests the getUserData function with an invalid URL
func TestInvalidGetUserDataInvalidUrl(t *testing.T) {

	// Create a context
	ctx := context.Background()

	// Send request to invalid URL
	reqPath := "invalidURL"
	// Call the getUserData function
	userData, _ := azure.GetUserData(ctx, reqPath)

	// Check that the userData is empty
	if userData != nil {
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
	userData, _ := azure.GetUserData(ctx, reqPath)

	// Check that the userData is empty
	if userData != nil {
		t.Fatalf("getUserData returned non-empty userData")
	}
}

type TestProvider struct {
	content  string
	failNext bool
}

func (p *TestProvider) GetUserData(ctx context.Context) ([]byte, error) {
	if p.failNext {
		p.failNext = false
		return []byte("%$#"), nil
	}
	return []byte(p.content), nil
}

func (p *TestProvider) GetRetryDelay() time.Duration {
	return 1 * time.Millisecond
}

// TestGetCloudConfig tests retrieving and parsing of a daemon config
func TestGetCloudConfig(t *testing.T) {
	provider := TestProvider{content: "write_files: []"}
	_, err := getCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve and parse empty cloud config: %v", err)
	}

	provider = TestProvider{failNext: true, content: "write_files: []"}
	_, err = getCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	provider = TestProvider{content: `#cloud-config
write_files:
- path: /test
  content: |
    test
    test`}
	_, err = getCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve and parse valid cloud config: %v", err)
	}
}

// TestProcessCloudConfig fail tests
func TestFailProcessCloudConfig(t *testing.T) {
	content := "#cloud-config\nwrite_files:\n- path: /wrong\n  content: bla"
	provider := TestProvider{content: content}
	cc, err := getCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve and parse cloud config: %v", err)
	}
	err = processCloudConfig(cc)
	if err == nil {
		t.Fatalf("it should fail as there is no file w/ $daemonConfigPath")
	}
}

// TestProcessCloudConfig tests parsing and provisioning of a daemon config
func TestProcessCloudConfig(t *testing.T) {
	// create temporary daemon config file
	tmpDaemonConfigFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpDaemonConfigFile.Name())
	cfg.daemonConfigPath = tmpDaemonConfigFile.Name()

	// create temporary auth json file
	tmpAuthJsonFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpAuthJsonFile.Name())
	cfg.authJsonPath = tmpAuthJsonFile.Name()

	// embed daemon config fixture in cloud config
	indented := strings.ReplaceAll(testDaemonConfig, "\n", "\n    ")
	content := fmt.Sprintf("#cloud-config\nwrite_files:\n- path: %s\n  content: |\n    %s", cfg.daemonConfigPath, indented)
	provider := TestProvider{content: content}
	cc, err := getCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve and parse cloud config: %v", err)
	}

	// process cloud config
	err = processCloudConfig(cc)
	if err != nil {
		t.Fatalf("failed to process cloud config: %v", err)
	}

	// check if files have been written correctly
	data, err := os.ReadFile(tmpDaemonConfigFile.Name())
	if err != nil {
		t.Fatalf("failed to read daemon config file: %v", err)
	}
	fileContent := string(data)

	if fileContent != testDaemonConfig {
		t.Fatalf("file content does not match daemon config fixture: got %q", fileContent)
	}

	data, err = os.ReadFile(tmpAuthJsonFile.Name())
	fileContent = string(data)
	if err != nil {
		t.Fatalf("failed to read auth json file: %v", err)
	}

	if fileContent != `{"auths":{}}` {
		t.Fatalf("file content does not match auth json fixture: got %q", fileContent)
	}
}

// TestFailPlainTextUserData tests with plain text userData
func TestFailPlainTextUserData(t *testing.T) {
	// startTestServerPlainText
	srv := startTestServerPlainText()
	defer srv.Close()

	// Create a context
	ctx := context.Background()

	// Send request to srv.URL at path /metadata/instance/compute/userData

	reqPath := srv.URL + "/metadata/instance/compute/userData"
	// Call the getUserData function
	userData, _ := azure.GetUserData(ctx, reqPath)

	// Check that the userData is empty. Since plain text userData is not supported
	if userData != nil {
		t.Fatalf("getUserData returned userData")
	}

}

func TestParseDaemonConfig(t *testing.T) {
	// Get the config from the test data
	config, err := parseDaemonConfig([]byte(testDaemonConfig))
	if err != nil {
		t.Fatalf("parseDaemonConfig failed: %v", err)
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

// Test the writeAgentConfig function
func TestWriteAgentConfig(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary file for the test
	tmpFile, err := os.CreateTemp(tmpDir, "agent-config.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	defer os.Remove(tmpFile.Name())

	// Create an instance of AgentConfig
	agentConfig := AgentConfig{
		// Set the fields of AgentConfig
		EnableSignatureVerification: true,
		ServerAddr:                  "unix:///run/kata-containers/agent.sock",
		AaKbcParams:                 "cc_kbc::http://192.168.1.2:8080",
		ImageRegistryAuthFile:       "/etc/attestation-agent/auth.json",
		Endpoints:                   Endpoints{Allowed: []string{"AddARPNeighborsRequest", "AddSwapRequest"}},
	}

	// Call the writeAgentConfig function
	err = writeAgentConfig(agentConfig, tmpFile.Name())
	assert.NoError(t, err)

	// Parse the agent config file data
	tmpAgentConfig, err := parseAgentConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to parse agent config file: %v", err)
	}

	// Use deepequal to match agentConfig and tmpAgentConfig
	assert.Equal(t, agentConfig, *tmpAgentConfig)

}

// Test the parseAgentConfig function
func TestParseAgentConfig(t *testing.T) {

	// Parse the agent config file data
	agentConfig, err := parseAgentConfig("test-data/sample-agent-config.toml")
	if err != nil {
		t.Fatalf("failed to parse agent config file: %v", err)
	}

	// Verify that the config fields match the test data
	if agentConfig.EnableSignatureVerification != false {
		t.Fatalf("agentConfig.EnableSignatureVerification does not match test data: expected %v, got %v", false, agentConfig.EnableSignatureVerification)
	}

	if agentConfig.ServerAddr != "unix:///run/kata-containers/agent.sock" {
		t.Fatalf("agentConfig.ServerAddr does not match test data: expected %v, got %v", "unix:///run/kata-containers/agent.sock", agentConfig.ServerAddr)
	}

	if agentConfig.AaKbcParams != "" {
		t.Fatalf("agentConfig.AaKbcParams does not match test data: expected %v, got %v", "", agentConfig.AaKbcParams)
	}

	if agentConfig.ImageRegistryAuthFile != "file:///etc/attestation-agent/auth.json" {
		t.Fatalf("agentConfig.ImageRegistryAuthFile does not match test data: expected %v, got %v", "/etc/attestation-agent/auth.json", agentConfig.ImageRegistryAuthFile)
	}

	if agentConfig.Endpoints.Allowed[0] != "AddARPNeighborsRequest" {
		t.Fatalf("agentConfig.Endpoints does not match test data: expected %v, got %v", "AddARPNeighborsRequest", agentConfig.Endpoints.Allowed[0])
	}

	if agentConfig.Endpoints.Allowed[1] != "AddSwapRequest" {
		t.Fatalf("agentConfig.Endpoints does not match test data: expected %v, got %v", "AddSwapRequest", agentConfig.Endpoints.Allowed[1])
	}

}

// Test the writeAgentConfig function with non existent toml entry in agent config file
func TestWriteAgentConfigNonExistentTomlEntry(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary file for the test
	tmpFile, err := os.CreateTemp(tmpDir, "agent-config.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	defer os.Remove(tmpFile.Name())

	// Create an instance of AgentConfig
	agentConfig := AgentConfig{
		// Set the fields of AgentConfig
		EnableSignatureVerification: true,
		ServerAddr:                  "unix:///run/kata-containers/agent.sock",
		AaKbcParams:                 "cc_kbc::http://192.168.1.2:8080",
	}

	// Call the writeAgentConfig function
	err = writeAgentConfig(agentConfig, tmpFile.Name())
	assert.NoError(t, err)

	// Parse the agent config file data
	newAgentConfig, err := parseAgentConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to parse agent config file: %v", err)
	}

	// Add the missing field to the agentConfig
	newAgentConfig.ImageRegistryAuthFile = "file:///etc/attestation-agent/auth.json"
	newAgentConfig.Endpoints.Allowed = []string{""}

	// Update existing field
	newAgentConfig.AaKbcParams = "cc_kbc::offline_kbc"

	// Call the writeAgentConfig function
	err = writeAgentConfig(*newAgentConfig, tmpFile.Name())
	assert.NoError(t, err)

	// Parse the agent config file data
	tmpAgentConfig, err := parseAgentConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to parse agent config file: %v", err)
	}

	// Check if tmpAgentConfig has the new fields
	assert.Equal(t, newAgentConfig, tmpAgentConfig)
}
