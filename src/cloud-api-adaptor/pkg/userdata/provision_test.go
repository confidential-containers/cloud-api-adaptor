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
)

var testAPFConfig string = `{
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
kbs_cert = """
-----BEGIN CERTIFICATE-----
MIIFTDCCAvugAwIBAgIBADBGBgkqhkiG9w0BAQowOaAPMA0GCWCGSAFlAwQCAgUA
oRwwGgYJKoZIhvcNAQEIMA0GCWCGSAFlAwQCAgUAogMCATCjAwIBATB7MRQwEgYD
VQQLDAtFbmdpbmVlcmluZzELMAkGA1UEBhMCVVMxFDASBgNVBAcMC1NhbnRhIENs
YXJhMQswCQYDVQQIDAJDQTEfMB0GA1UECgwWQWR2YW5jZWQgTWljcm8gRGV2aWNl
czESMBAGA1UEAwwJU0VWLU1pbGFuMB4XDTIzMDEyNDE3NTgyNloXDTMwMDEyNDE3
NTgyNlowejEUMBIGA1UECwwLRW5naW5lZXJpbmcxCzAJBgNVBAYTAlVTMRQwEgYD
VQQHDAtTYW50YSBDbGFyYTELMAkGA1UECAwCQ0ExHzAdBgNVBAoMFkFkdmFuY2Vk
IE1pY3JvIERldmljZXMxETAPBgNVBAMMCFNFVi1WQ0VLMHYwEAYHKoZIzj0CAQYF
K4EEACIDYgAExmG1ZbuoAQK93USRyZQcsyobfbaAEoKEELf/jK39cOVJt1t4s83W
XM3rqIbS7qHUHQw/FGyOvdaEUs5+wwxpCWfDnmJMAQ+ctgZqgDEKh1NqlOuuKcKq
2YAWE5cTH7sHo4IBFjCCARIwEAYJKwYBBAGceAEBBAMCAQAwFwYJKwYBBAGceAEC
BAoWCE1pbGFuLUIwMBEGCisGAQQBnHgBAwEEAwIBAzARBgorBgEEAZx4AQMCBAMC
AQAwEQYKKwYBBAGceAEDBAQDAgEAMBEGCisGAQQBnHgBAwUEAwIBADARBgorBgEE
AZx4AQMGBAMCAQAwEQYKKwYBBAGceAEDBwQDAgEAMBEGCisGAQQBnHgBAwMEAwIB
CDARBgorBgEEAZx4AQMIBAMCAXMwTQYJKwYBBAGceAEEBEDDhCejDUx6+dlvehW5
cmmCWmTLdqI1L/1dGBFdia1HP46MC82aXZKGYSutSq37RCYgWjueT+qCMBE1oXDk
d1JOMEYGCSqGSIb3DQEBCjA5oA8wDQYJYIZIAWUDBAICBQChHDAaBgkqhkiG9w0B
AQgwDQYJYIZIAWUDBAICBQCiAwIBMKMDAgEBA4ICAQACgCai9x8DAWzX/2IelNWm
ituEBSiq9C9eDnBEckQYikAhPasfagnoWFAtKu/ZWTKHi+BMbhKwswBS8W0G1ywi
cUWGlzigI4tdxxf1YBJyCoTSNssSbKmIh5jemBfrvIBo1yEd+e56ZJMdhN8e+xWU
bvovUC2/7Dl76fzAaACLSorZUv5XPJwKXwEOHo7FIcREjoZn+fKjJTnmdXce0LD6
9RHr+r+ceyE79gmK31bI9DYiJoL4LeGdXZ3gMOVDR1OnDos5lOBcV+quJ6JujpgH
d9g3Sa7Du7pusD9Fdap98ocZslRfFjFi//2YdVM4MKbq6IwpYNB+2PCEKNC7SfbO
NgZYJuPZnM/wViES/cP7MZNJ1KUKBI9yh6TmlSsZZOclGJvrOsBZimTXpATjdNMt
cluKwqAUUzYQmU7bf2TMdOXyA9iH5wIpj1kWGE1VuFADTKILkTc6LzLzOWCofLxf
onhTtSDtzIv/uel547GZqq+rVRvmIieEuEvDETwuookfV6qu3D/9KuSr9xiznmEg
xynud/f525jppJMcD/ofbQxUZuGKvb3f3zy+aLxqidoX7gca2Xd9jyUy5Y/83+ZN
bz4PZx81UJzXVI9ABEh8/xilATh1ZxOePTBJjN7lgr0lXtKYjV/43yyxgUYrXNZS
oLSG2dLCK9mjjraPjau34Q==
-----END CERTIFICATE-----
"""
`

var testAAConfig string = `[token_configs]
[token_configs.coco_as]
url = 'http://127.0.0.1:8080'

[token_configs.kbs]
url = 'http://127.0.0.1:8080'
cert = """
-----BEGIN CERTIFICATE-----
MIIDljCCAn6gAwIBAgIUR/UNh13GFam4emgludtype/S9BIwDQYJKoZIhvcNAQEL
BQAwdTELMAkGA1UEBhMCQ04xETAPBgNVBAgMCFpoZWppYW5nMREwDwYDVQQHDAhI
YW5nemhvdTERMA8GA1UECgwIQUFTLVRFU1QxFDASBgNVBAsMC0RldmVsb3BtZW50
MRcwFQYDVQQDDA5BQVMtVEVTVC1IVFRQUzAeFw0yNDAzMTgwNzAzNTNaFw0yNTAz
MTgwNzAzNTNaMHUxCzAJBgNVBAYTAkNOMREwDwYDVQQIDAhaaGVqaWFuZzERMA8G
A1UEBwwISGFuZ3pob3UxETAPBgNVBAoMCEFBUy1URVNUMRQwEgYDVQQLDAtEZXZl
bG9wbWVudDEXMBUGA1UEAwwOQUFTLVRFU1QtSFRUUFMwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQDfp1aBr6LiNRBlJUcDGcAbcUCPG6UzywtVIc8+comS
ay//gwz2AkDmFVvqwI4bdp/NUCwSC6ShHzxsrCEiagRKtA3af/ckM7hOkb4S6u/5
ewHHFcL6YOUp+NOH5/dSLrFHLjet0dt4LkyNBPe7mKAyCJXfiX3wb25wIBB0Tfa0
p5VoKzwWeDQBx7aX8TKbG6/FZIiOXGZdl24DGARiqE3XifX7DH9iVZ2V2RL9+3WY
05GETNFPKtcrNwTy8St8/HsWVxjAzGFzf75Lbys9Ff3JMDsg9zQzgcJJzYWisxlY
g3CmnbENP0eoHS4WjQlTUyY0mtnOwodo4Vdf8ZOkU4wJAgMBAAGjHjAcMBoGA1Ud
EQQTMBGCCWxvY2FsaG9zdIcEfwAAATANBgkqhkiG9w0BAQsFAAOCAQEAKW32spii
t2JB7C1IvYpJw5mQ5bhIlldE0iB5rwWvNbuDgPrgfTI4xiX5sumdHw+P2+GU9KXF
nWkFRZ9W/26xFrVgGIS/a07aI7xrlp0Oj+1uO91UhCL3HhME/0tPC6z1iaFeZp8Y
T1tLnafqiGiThFUgvg6PKt86enX60vGaTY7sslRlgbDr9sAi/NDSS7U1PviuC6yo
yJi7BDiRSx7KrMGLscQ+AKKo2RF1MLzlJMa1kIZfvKDBXFzRd61K5IjDRQ4HQhwX
DYEbQvoZIkUTc1gBUWDcAUS5ztbJg9LCb9WVtvUTqTP2lGuNymOvdsuXq+sAZh9b
M9QaC1mzQ/OStg==
-----END CERTIFICATE-----
"""
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

var testCheckSum = "52af3178dd7ad4bf551e629b84b45bfd1fbe1434b980120267181ae3575ea20ca9013b8eadf31d27eed7ff2552d500ef"
var cc_init_data = "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"

var testScratchSpaceEnv string = ""

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
func TestIMDSGet(t *testing.T) {
	// Start a temporary HTTP server for the test simulating
	// the Azure metadata service
	srv := startTestServer()
	defer srv.Close()

	// Create a context
	ctx := context.Background()

	// Send request to srv.URL at path /metadata/instance/compute/userData

	reqPath := srv.URL + "/metadata/instance/compute/userData"
	userData, _ := imdsGet(ctx, reqPath, true, nil)

	// Check that the userData is not empty
	if userData == nil {
		t.Fatalf("getUserData returned empty userData")
	}
}

// TestInvalidGetUserDataInvalidUrl tests the getUserData function with an invalid URL
func TestInvalidIMDSGetInvalidUrl(t *testing.T) {

	// Create a context
	ctx := context.Background()

	// Send request to invalid URL
	reqPath := "invalidURL"
	userData, _ := imdsGet(ctx, reqPath, true, nil)

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
	userData, _ := imdsGet(ctx, reqPath, true, nil)

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

// TestRetrieveCloudConfig tests retrieving and parsing of a apf config
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

	var aaCfgPath = filepath.Join(tempDir, "aa.toml")
	var cdhCfgPath = filepath.Join(tempDir, "cdh.toml")
	var apfCfgPath = filepath.Join(tempDir, "apf.json")
	var authPath = filepath.Join(tempDir, "auth.json")
	var initdataPath = filepath.Join(tempDir, "initdata")
	var scratchSpacePath = filepath.Join(tempDir, "scratch-space.marker")
	var writeFilesList = []string{aaCfgPath, cdhCfgPath, apfCfgPath, authPath, initdataPath, scratchSpacePath}

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
- path: %s
  content: |
%s
- path: %s
  content: |
%s
`,
		aaCfgPath,
		indentTextBlock(testAAConfig, 4),
		cdhCfgPath,
		indentTextBlock(testCDHConfig, 4),
		apfCfgPath,
		indentTextBlock(testAPFConfig, 4),
		authPath,
		indentTextBlock(testAuthJson, 4),
		initdataPath,
		indentTextBlock(cc_init_data, 4),
		scratchSpacePath,
		indentTextBlock(testScratchSpaceEnv, 4),
	)

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
		writeFiles:    writeFilesList,
		initdataFiles: nil,
	}
	if err := processCloudConfig(&cfg, cc); err != nil {
		t.Fatalf("failed to process cloud config file: %v", err)
	}

	// check if files have been written correctly
	data, _ := os.ReadFile(aaCfgPath)
	fileContent := string(data)
	if fileContent != testAAConfig {
		t.Fatalf("file content does not match aa config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(cdhCfgPath)
	fileContent = string(data)
	if fileContent != testCDHConfig {
		t.Fatalf("file content does not match cdh config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(apfCfgPath)
	fileContent = string(data)
	if fileContent != testAPFConfig {
		t.Fatalf("file content does not match apf config fixture: got %q", fileContent)
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

	var aaCfgPath = filepath.Join(tempDir, "aa.toml")
	var cdhCfgPath = filepath.Join(tempDir, "cdh.toml")
	var apfCfgPath = filepath.Join(tempDir, "apf.json")
	var authPath = filepath.Join(tempDir, "auth.json")
	var malicious = filepath.Join(tempDir, "malicious")
	var writeFilesList = []string{aaCfgPath, cdhCfgPath, apfCfgPath, authPath}

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
- path: %s
  content: |
%s
`,
		aaCfgPath,
		indentTextBlock(testAAConfig, 4),
		cdhCfgPath,
		indentTextBlock(testCDHConfig, 4),
		apfCfgPath,
		indentTextBlock(testAPFConfig, 4),
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
		writeFiles:    writeFilesList,
		initdataFiles: nil,
	}
	if err := processCloudConfig(&cfg, cc); err != nil {
		t.Fatalf("failed to process cloud config file: %v", err)
	}

	// check if files have been written correctly
	data, _ := os.ReadFile(aaCfgPath)
	fileContent := string(data)
	if fileContent != testAAConfig {
		t.Fatalf("file content does not match aa config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(cdhCfgPath)
	fileContent = string(data)
	if fileContent != testCDHConfig {
		t.Fatalf("file content does not match cdh config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(apfCfgPath)
	fileContent = string(data)
	if fileContent != testAPFConfig {
		t.Fatalf("file content does not match apf config fixture: got %q", fileContent)
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
	userData, _ := imdsGet(ctx, reqPath, true, nil)

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
	var initdDataFilesList = []string{aaPath, cdhPath, policyPath}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    digestPath,
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    nil,
		initdataFiles: initdDataFilesList,
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

func TestWithoutInitdata(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_dummy_initdata_root")
	defer os.RemoveAll(tempDir)

	var initdataPath = filepath.Join(tempDir, "initdata")
	var aaPath = filepath.Join(tempDir, "aa.toml")
	var digestPath = filepath.Join(tempDir, "initdata.digest")
	var initdDataFilesList = []string{aaPath}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    digestPath,
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    nil,
		initdataFiles: initdDataFilesList,
	}

	err := extractInitdataAndHash(&cfg)
	if err != nil {
		t.Fatalf("extractInitdataAndHash returned err: %v", err)
	}

	// Verify dummy initdata file was created
	if _, err := os.Stat(initdataPath); os.IsNotExist(err) {
		t.Fatalf("dummy initdata file was not created")
	}

	// Verify aa.toml was created with empty content
	bytes, err := os.ReadFile(aaPath)
	if err != nil {
		t.Fatalf("failed to read aa.toml: %v", err)
	}
	if string(bytes) != "" {
		t.Fatalf("aa.toml should be empty but got: %s", string(bytes))
	}

	// Verify digest was created
	digestBytes, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}
	if len(digestBytes) == 0 {
		t.Fatalf("digest file should not be empty")
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
	var initdDataFilesList = []string{aaPath, cdhPath, policyPath}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    digestPath,
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    nil,
		initdataFiles: initdDataFilesList,
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
