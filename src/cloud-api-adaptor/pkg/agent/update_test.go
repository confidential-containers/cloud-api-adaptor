package agent

import (
	"fmt"
	"os"
	"strings"
	"testing"

	daemon "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/tj/assert"
)

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
