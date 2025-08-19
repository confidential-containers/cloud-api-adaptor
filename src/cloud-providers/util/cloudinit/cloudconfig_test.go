// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cloudinit

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

const forwarderConfigPath = "/peerpod/apf.json"
const authJSONPath = "/run/peerpod/auth.json"

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

// Add a test to create a cloud-init config with apf.json and auth.json file
// The test should verify that the config has both the apf.json and the auth.json
// files in the write_files section.
func TestUserDataWithAPFConfigAndAuth(t *testing.T) {
	testAPFConfigJSON := `{
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
	testAuthJSON := `{
				"auths": {
						"myregistry.io": {
								"auth": "dXNlcjpwYXNzd29yZAo"
						}
				}
		}`

	testResourcesJSON := AuthJSONToResourcesJSON(string(testAuthJSON))

	// Write tempAPFConfigJSON to cloud-init config file
	// Create a CloudConfig struct
	cloudConfig := &CloudConfig{
		WriteFiles: []WriteFile{
			{Path: forwarderConfigPath, Content: string(testAPFConfigJSON)},
			{Path: authJSONPath, Content: testResourcesJSON},
		},
	}

	// Generate userData from cloudConfig
	userData, err := cloudConfig.Generate()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	// Pretty print the userData
	fmt.Printf("userData: %s\n", userData)

	// Verify that the userData has the apf.json and auth.json files
	// in the write_files section
	if !strings.Contains(userData, forwarderConfigPath) {
		t.Fatalf("Expect %q, got %q", forwarderConfigPath, userData)
	}

	if !strings.Contains(userData, authJSONPath) {
		t.Fatalf("Expect %q, got %q", authJSONPath, userData)
	}

	var output CloudConfig

	if err := yaml.Unmarshal([]byte(userData), &output); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	// Pretty print the userData output
	fmt.Printf("userData: %s\n", output)

	// Verify that the output yaml has the testAPFConfigJson and testb64AuthJson contents
	// in the write_files section
	if !strings.Contains(output.WriteFiles[0].Content, testAPFConfigJSON) {
		t.Fatalf("Expect %q, got %q", testAPFConfigJSON, output.WriteFiles[0].Content)
	}

	if !strings.Contains(output.WriteFiles[1].Content, testResourcesJSON) {
		t.Fatalf("Expect %q, got %q", testResourcesJSON, output.WriteFiles[1].Content)
	}

}
