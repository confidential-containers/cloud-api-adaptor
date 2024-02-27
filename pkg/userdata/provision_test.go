package userdata

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

	"github.com/confidential-containers/cloud-api-adaptor/pkg/userdata/azure"
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
		t.Fatalf("couldn't retrieve and parse valid cloud config: %v", err)
	}
}

// TestProcessCloudConfig fail tests
func TestFailProcessCloudConfig(t *testing.T) {
	content := "#cloud-config\nwrite_files:\n- path: /wrong\n  content: bla"
	provider := TestProvider{content: content}
	cc, err := retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve and parse cloud config: %v", err)
	}
	_, _, err = findDaemonConfigEntry("/other", cc)
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

	// create temporary auth json file
	tmpAuthJsonFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpAuthJsonFile.Name())

	// embed daemon config fixture in cloud config
	indented := strings.ReplaceAll(testDaemonConfig, "\n", "\n    ")
	content := fmt.Sprintf("#cloud-config\nwrite_files:\n- path: %s\n  content: |\n    %s", tmpDaemonConfigFile.Name(), indented)
	provider := TestProvider{content: content}

	cc, err := retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve and parse cloud config: %v", err)
	}

	cfg := Config{
		daemonConfigPath: tmpDaemonConfigFile.Name(),
		authJsonPath:     tmpAuthJsonFile.Name(),
	}
	if err := processCloudConfig(&cfg, cc); err != nil {
		t.Fatalf("failed to process cloud config file: %v", err)
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

	data, _ = os.ReadFile(tmpAuthJsonFile.Name())
	fileContent = string(data)

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
