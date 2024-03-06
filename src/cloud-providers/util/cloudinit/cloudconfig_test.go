// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cloudinit

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

const forwarderConfigPath = "/peerpod/daemon.json"

func TestUserData(t *testing.T) {
	cloudConfig := &CloudConfig{
		WriteFiles: []WriteFile{
			{Path: "/123", Content: "Hello\n"},
			{Path: "/456", Content: "Hello\nWorld\n", Owner: "root:root"},
		},
	}

	userData, err := cloudConfig.Generate()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	firstLine := userData[0:strings.Index(userData, "\n")]

	if e, a := "#cloud-config", firstLine; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}

	var output CloudConfig

	if err := yaml.Unmarshal([]byte(userData), &output); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	if e, a := cloudConfig, &output; !reflect.DeepEqual(e, a) {
		t.Fatalf("Expect %#v, got %#v", e, a)
	}
}

// Add a test to create a cloud-init config with daemon.json and auth.json file
// The test should verify that the config has both the daemon.json and the auth.json
// files in the write_files section.
func TestUserDataWithDaemonAndAuth(t *testing.T) {
	testDaemonConfigJson := `{
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
	}`

	// Create a variable to hold sample base64 encoded string which is the auth.json
	// file
	testAuthJson := `{
				"auths": {
						"myregistry.io": {
								"auth": "dXNlcjpwYXNzd29yZAo"
						}
				}
		}`

	testResourcesJson := AuthJSONToResourcesJSON(string(testAuthJson))

	// Write tempDaemonConfigJSON to cloud-init config file
	// Create a CloudConfig struct
	cloudConfig := &CloudConfig{
		WriteFiles: []WriteFile{
			{Path: forwarderConfigPath, Content: string(testDaemonConfigJson)},
			{Path: DefaultAuthfileDstPath, Content: testResourcesJson},
		},
	}

	// Generate userData from cloudConfig
	userData, err := cloudConfig.Generate()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	// Pretty print the userData
	fmt.Printf("userData: %s\n", userData)

	// Verify that the userData has the daemon.json and auth.json files
	// in the write_files section
	if !strings.Contains(userData, forwarderConfigPath) {
		t.Fatalf("Expect %q, got %q", forwarderConfigPath, userData)
	}

	if !strings.Contains(userData, DefaultAuthfileDstPath) {
		t.Fatalf("Expect %q, got %q", DefaultAuthfileDstPath, userData)
	}

	var output CloudConfig

	if err := yaml.Unmarshal([]byte(userData), &output); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	// Pretty print the userData output
	fmt.Printf("userData: %s\n", output)

	// Verify that the output yaml has the testDaemonConfigJson and testb64AuthJson contents
	// in the write_files section
	if !strings.Contains(output.WriteFiles[0].Content, testDaemonConfigJson) {
		t.Fatalf("Expect %q, got %q", testDaemonConfigJson, output.WriteFiles[0].Content)
	}

	if !strings.Contains(output.WriteFiles[1].Content, testResourcesJson) {
		t.Fatalf("Expect %q, got %q", testResourcesJson, output.WriteFiles[1].Content)
	}

}

// Test userData with a daemon.json file, an auth.json file and
// kbc-params.
// The test should verify that the config has the daemon.json, auth.json and kbc-params
// files in the write_files section.
func TestUserDataWithDaemonAndAuthAndAAKBCParams(t *testing.T) {
	testDaemonConfigJson := `{
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

	// Create a variable to hold sample base64 encoded string which is the auth.json
	// file
	testAuthJson := `{
				"auths": {
						"myregistry.io": {
								"auth": "dXNlcjpwYXNzd29yZAo"
						}
				}
		}`

	testResourcesJson := AuthJSONToResourcesJSON(string(testAuthJson))

	// Create a CloudConfig struct
	cloudConfig := &CloudConfig{
		WriteFiles: []WriteFile{
			{Path: forwarderConfigPath, Content: string(testDaemonConfigJson)},
			{Path: DefaultAuthfileDstPath, Content: testResourcesJson},
		},
	}

	// Generate userData from cloudConfig
	userData, err := cloudConfig.Generate()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	// Pretty print the userData
	fmt.Printf("userData: %s\n", userData)

	// Verify that the userData has the daemon.json, auth.json and kbc-params files
	// in the write_files section
	if !strings.Contains(userData, forwarderConfigPath) {
		t.Fatalf("Expect %q, got %q", forwarderConfigPath, userData)
	}

	if !strings.Contains(userData, DefaultAuthfileDstPath) {
		t.Fatalf("Expect %q, got %q", DefaultAuthfileDstPath, userData)
	}

	var output CloudConfig

	if err := yaml.Unmarshal([]byte(userData), &output); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	// Pretty print the userData output
	fmt.Printf("userData: %s\n", output)

	// Verify that the output yaml has the testDaemonConfigJson, testb64AuthJson and testAAKBCParams contents
	// in the write_files section
	if !strings.Contains(output.WriteFiles[0].Content, testDaemonConfigJson) {
		t.Fatalf("Expect %q, got %q", testDaemonConfigJson, output.WriteFiles[0].Content)
	}

	if !strings.Contains(output.WriteFiles[1].Content, testResourcesJson) {
		t.Fatalf("Expect %q, got %q", testResourcesJson, output.WriteFiles[1].Content)
	}

}
