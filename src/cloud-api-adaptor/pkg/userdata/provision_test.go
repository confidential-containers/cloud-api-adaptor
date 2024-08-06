package userdata

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/azure"
)

var testAgentConfig string = `server_addr = 'unix:///run/kata-containers/agent.sock'
guest_components_procs = 'none'
`

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
	"tls-client-ca": "-----BEGIN CERTIFICATE-----\n....\n-----END CERTIFICATE-----\n"
}
`

var testAuthJson string = `{
	"auths":{}
}
`

var testCDHConfig string = `socket = 'unix:///run/confidential-containers/cdh.sock'
credentials = []

[kbc]
name = 'cc_kbc'
url = 'http://1.2.3.4:8080'
`

var testAAConfig string = `[token_configs]
[token_configs.coco_as]
url = 'http://127.0.0.1:8080'

[token_configs.kbs]
url = 'http://127.0.0.1:8080'
`

var testPolicyConfig string = `package agent_policy

import future.keywords.in
import future.keywords.every

import input

# Default values, returned by OPA when rules cannot be evaluated to true.
default CopyFileRequest := false
default CreateContainerRequest := false
default CreateSandboxRequest := true
default DestroySandboxRequest := true
default ExecProcessRequest := false
default GetOOMEventRequest := true
default GuestDetailsRequest := true
default OnlineCPUMemRequest := true
default PullImageRequest := true
default ReadStreamRequest := false
default RemoveContainerRequest := true
default RemoveStaleVirtiofsShareMountsRequest := true
default SignalProcessRequest := true
default StartContainerRequest := true
default StatsContainerRequest := true
default TtyWinResizeRequest := true
default UpdateEphemeralMountsRequest := true
default UpdateInterfaceRequest := true
default UpdateRoutesRequest := true
default WaitProcessRequest := true
default WriteStreamRequest := false
`

var testCheckSum = "14980c75860de9adcba2e0e494fc612f0f4fe3d86f5dc8e238a3255acfdf43bf82b9ccfc21da95d639ff0c98cc15e05e"

var cc_init_data = "YWxnb3JpdGhtID0gInNoYTM4NCIKdmVyc2lvbiA9ICIwLjEuMCIKCltkYXRhXQoiYWEudG9tbCIgPSAnJycKW3Rva2VuX2NvbmZpZ3NdClt0b2tlbl9jb25maWdzLmNvY29fYXNdCnVybCA9ICdodHRwOi8vMTI3LjAuMC4xOjgwODAnCgpbdG9rZW5fY29uZmlncy5rYnNdCnVybCA9ICdodHRwOi8vMTI3LjAuMC4xOjgwODAnCicnJwoKImNkaC50b21sIiAgPSAnJycKc29ja2V0ID0gJ3VuaXg6Ly8vcnVuL2NvbmZpZGVudGlhbC1jb250YWluZXJzL2NkaC5zb2NrJwpjcmVkZW50aWFscyA9IFtdCgpba2JjXQpuYW1lID0gJ2NjX2tiYycKdXJsID0gJ2h0dHA6Ly8xLjIuMy40OjgwODAnCicnJwoKInBvbGljeS5yZWdvIiA9ICcnJwpwYWNrYWdlIGFnZW50X3BvbGljeQoKaW1wb3J0IGZ1dHVyZS5rZXl3b3Jkcy5pbgppbXBvcnQgZnV0dXJlLmtleXdvcmRzLmV2ZXJ5CgppbXBvcnQgaW5wdXQKCiMgRGVmYXVsdCB2YWx1ZXMsIHJldHVybmVkIGJ5IE9QQSB3aGVuIHJ1bGVzIGNhbm5vdCBiZSBldmFsdWF0ZWQgdG8gdHJ1ZS4KZGVmYXVsdCBDb3B5RmlsZVJlcXVlc3QgOj0gZmFsc2UKZGVmYXVsdCBDcmVhdGVDb250YWluZXJSZXF1ZXN0IDo9IGZhbHNlCmRlZmF1bHQgQ3JlYXRlU2FuZGJveFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IERlc3Ryb3lTYW5kYm94UmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgRXhlY1Byb2Nlc3NSZXF1ZXN0IDo9IGZhbHNlCmRlZmF1bHQgR2V0T09NRXZlbnRSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBHdWVzdERldGFpbHNSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBPbmxpbmVDUFVNZW1SZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBQdWxsSW1hZ2VSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBSZWFkU3RyZWFtUmVxdWVzdCA6PSBmYWxzZQpkZWZhdWx0IFJlbW92ZUNvbnRhaW5lclJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFJlbW92ZVN0YWxlVmlydGlvZnNTaGFyZU1vdW50c1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFNpZ25hbFByb2Nlc3NSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBTdGFydENvbnRhaW5lclJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFN0YXRzQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgVHR5V2luUmVzaXplUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgVXBkYXRlRXBoZW1lcmFsTW91bnRzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgVXBkYXRlSW50ZXJmYWNlUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgVXBkYXRlUm91dGVzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgV2FpdFByb2Nlc3NSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBXcml0ZVN0cmVhbVJlcXVlc3QgOj0gZmFsc2UKJycn"

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

// TestRetrieveCloudConfig tests retrieving and parsing of a daemon config
func TestRetrieveCloudConfig(t *testing.T) {
	var provider TestProvider

	provider = TestProvider{content: "write_files: []"}
	_, err := retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve and parse empty cloud config: %v", err)
	}

	provider = TestProvider{failNext: true, content: "write_files: []"}
	_, err = retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	provider = TestProvider{content: `#cloud-config
write_files:
- path: /test
  content: |
    test
    test`}
	_, err = retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve valid cloud config: %v", err)
	}
}

func indentTextBlock(text string, by int) string {
	whiteSpace := strings.Repeat(" ", by)
	split := strings.Split(text, "\n")
	indented := ""
	for _, line := range split {
		indented += whiteSpace + line + "\n"
	}
	return indented
}

func TestProcessCloudConfig(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_writefiles_root")
	defer os.RemoveAll(tempDir)

	var agentCfgPath = filepath.Join(tempDir, "agent-config.toml")
	var daemonPath = filepath.Join(tempDir, "daemon.json")
	var authPath = filepath.Join(tempDir, "auth.json")
	var initdataPath = filepath.Join(tempDir, "initdata")

	content := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
`,
		agentCfgPath,
		indentTextBlock(testAgentConfig, 4),
		daemonPath,
		indentTextBlock(testDaemonConfig, 4),
		authPath,
		indentTextBlock(testAuthJson, 4),
		initdataPath,
		indentTextBlock(cc_init_data, 4))

	provider := TestProvider{content: content}

	cc, err := retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve cloud config: %v", err)
	}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    "",
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    WriteFilesList,
		initdataFiles: nil,
	}
	if err := processCloudConfig(&cfg, cc); err != nil {
		t.Fatalf("failed to process cloud config file: %v", err)
	}

	// check if files have been written correctly
	data, _ := os.ReadFile(agentCfgPath)
	fileContent := string(data)
	if fileContent != testAgentConfig {
		t.Fatalf("file content does not match daemon config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(daemonPath)
	fileContent = string(data)
	if fileContent != testDaemonConfig {
		t.Fatalf("file content does not match daemon config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(authPath)
	fileContent = string(data)
	if fileContent != testAuthJson {
		t.Fatalf("file content does not match auth json fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(initdataPath)
	fileContent = string(data)
	if fileContent != cc_init_data+"\n" {
		t.Fatalf("file content does not match initdata fixture: got %q", fileContent)
	}
}

func TestProcessCloudConfigWithMalicious(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_writefiles_root")
	defer os.RemoveAll(tempDir)

	var agentCfgPath = filepath.Join(tempDir, "agent-config.toml")
	var daemonPath = filepath.Join(tempDir, "daemon.json")
	var authPath = filepath.Join(tempDir, "auth.json")
	var malicious = filepath.Join(tempDir, "malicious")

	content := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
`,
		agentCfgPath,
		indentTextBlock(testAgentConfig, 4),
		daemonPath,
		indentTextBlock(testDaemonConfig, 4),
		authPath,
		indentTextBlock(testAuthJson, 4),
		malicious,
		indentTextBlock("malicious", 4))

	provider := TestProvider{content: content}

	cc, err := retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve cloud config: %v", err)
	}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    "",
		initdataPath:  "",
		parentPath:    tempDir,
		writeFiles:    WriteFilesList,
		initdataFiles: nil,
	}
	if err := processCloudConfig(&cfg, cc); err != nil {
		t.Fatalf("failed to process cloud config file: %v", err)
	}

	// check if files have been written correctly
	data, _ := os.ReadFile(agentCfgPath)
	fileContent := string(data)
	if fileContent != testAgentConfig {
		t.Fatalf("file content does not match daemon config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(daemonPath)
	fileContent = string(data)
	if fileContent != testDaemonConfig {
		t.Fatalf("file content does not match daemon config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(authPath)
	fileContent = string(data)
	if fileContent != testAuthJson {
		t.Fatalf("file content does not match auth json fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(malicious)
	if data != nil {
		t.Fatalf("file content should be nil but: got %q", string(data))
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

func TestExtractInitdataAndHash(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_initdata_root")
	defer os.RemoveAll(tempDir)

	var initdataPath = filepath.Join(tempDir, "initdata")
	var aaPath = filepath.Join(tempDir, "aa.toml")
	var cdhPath = filepath.Join(tempDir, "cdh.toml")
	var policyPath = filepath.Join(tempDir, "policy.rego")
	var digestPath = filepath.Join(tempDir, "initdata.digest")
	cfg := Config{
		fetchTimeout:  180,
		digestPath:    digestPath,
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    nil,
		initdataFiles: InitdDataFilesList,
	}

	_ = writeFile(initdataPath, []byte(cc_init_data))
	err := extractInitdataAndHash(&cfg)
	if err != nil {
		t.Fatalf("extractInitdataAndHash returned err: %v", err)
	}

	bytes, _ := os.ReadFile(aaPath)
	aaStr := string(bytes)
	if testAAConfig != aaStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", aaStr, testAAConfig)
	}

	bytes, _ = os.ReadFile(cdhPath)
	cdhStr := string(bytes)
	if testCDHConfig != cdhStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", cdhStr, testCDHConfig)
	}

	bytes, _ = os.ReadFile(policyPath)
	regoStr := string(bytes)
	if testPolicyConfig != regoStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", regoStr, testPolicyConfig)
	}

	bytes, _ = os.ReadFile(digestPath)
	sum := string(bytes)
	if testCheckSum != sum {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", sum, testCheckSum)
	}
}

func TestExtractInitdataWithMalicious(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_initdata_root")
	defer os.RemoveAll(tempDir)

	var initdataPath = filepath.Join(tempDir, "initdata")
	var aaPath = filepath.Join(tempDir, "aa.toml")
	var cdhPath = filepath.Join(tempDir, "cdh.toml")
	var policyPath = filepath.Join(tempDir, "malicious.rego")
	var digestPath = filepath.Join(tempDir, "initdata.digest")
	cfg := Config{
		fetchTimeout:  180,
		digestPath:    digestPath,
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    nil,
		initdataFiles: InitdDataFilesList,
	}

	_ = writeFile(initdataPath, []byte(cc_init_data))
	err := extractInitdataAndHash(&cfg)
	if err != nil {
		t.Fatalf("extractInitdataAndHash returned err: %v", err)
	}

	bytes, _ := os.ReadFile(aaPath)
	aaStr := string(bytes)
	if testAAConfig != aaStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", aaStr, testAAConfig)
	}

	bytes, _ = os.ReadFile(cdhPath)
	cdhStr := string(bytes)
	if testCDHConfig != cdhStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", cdhStr, testCDHConfig)
	}

	bytes, _ = os.ReadFile(policyPath)
	if bytes != nil {
		t.Fatalf("Should not read malicious file but got %s", string(bytes))
	}
}
