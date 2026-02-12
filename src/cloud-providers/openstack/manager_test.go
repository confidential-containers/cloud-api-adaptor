// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

type TestManager struct {
	*Manager
}

func TestParseCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected Config
	}{
		{
			name: "AllFlagsSet",
			args: []string{
				"-server-prefix=test-vm-name",
				"-imageID=test-image-id",
				"-flavorID=test-flavor-id",
				"-networkID=net-1,net-2,net-3",
				"-security-group=sg-1,sg-2,sg-3",
				"-floating-ip-networkID=floating-net",
				"-openstack-username=test-user",
				"-openstack-password=test-password",
				"-openstack-region=test-region",
				"-openstack-tenant-name=test-tenant",
				"-openstack-domain-name=test-domain",
				"-openstack-identity-endpoint=https://identity.testopenstack/v3",
			},
			expected: Config{
				ServerPrefix:        "test-vm-name",
				ImageID:             "test-image-id",
				FlavorID:            "test-flavor-id",
				NetworkIDs:          []string{"net-1", "net-2", "net-3"},
				SecurityGroups:      []string{"sg-1", "sg-2", "sg-3"},
				FloatingIpNetworkID: "floating-net",
				Username:            "test-user",
				Password:            "test-password",
				Region:              "test-region",
				TenantName:          "test-tenant",
				DomainName:          "test-domain",
				IdentityEndpoint:    "https://identity.testopenstack/v3",
			},
		},
		{
			name: "DefaultValues",
			args: []string{},
			expected: Config{
				ServerPrefix:        "",
				ImageID:             "",
				FlavorID:            "",
				NetworkIDs:          []string{},
				SecurityGroups:      []string{},
				FloatingIpNetworkID: "",
				Username:            "",
				Password:            "",
				Region:              "",
				TenantName:          "",
				DomainName:          "",
				IdentityEndpoint:    "",
			},
		},
		{
			name: "SingleNetworkAndSecurityGroup",
			args: []string{
				"-networkID=net-1",
				"-security-group=sg-1",
			},
			expected: Config{
				NetworkIDs:     []string{"net-1"},
				SecurityGroups: []string{"sg-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			testManager := &Manager{}
			testManager.ParseCmd(flags)
			err := flags.Parse(tt.args)

			if err != nil {
				t.Errorf("Failed to parse flags: %v", err)
			}

			if !comparestructs(openstackcfg, tt.expected) {
				t.Errorf("Expected config: %+v, but got: %+v\n", tt.expected, openstackcfg)
			} else {
				t.Logf("Expected config: %+v, got config: %+v\n", tt.expected, openstackcfg)
			}

			flags = nil
			openstackcfg = Config{}
		})
	}
}

func comparestructs(result, expected Config) bool {
	isEqual := true

	if result.ServerPrefix != expected.ServerPrefix {
		fmt.Printf("Expected ServerPrefix: %v, but got: %v\n", expected.ServerPrefix, result.ServerPrefix)
		isEqual = false
	}

	if result.ImageID != expected.ImageID {
		fmt.Printf("Expected ImageID: %v, but got: %v\n", expected.ImageID, result.ImageID)
		isEqual = false
	}

	if result.FlavorID != expected.FlavorID {
		fmt.Printf("Expected FlavorID: %v, but got: %v\n", expected.FlavorID, result.FlavorID)
		isEqual = false
	}

	if len(result.NetworkIDs) != len(expected.NetworkIDs) {
		fmt.Printf("Expected NetworkIDs length: %v, but got: %v\n", len(expected.NetworkIDs), len(result.NetworkIDs))
		isEqual = false
	} else {
		for i := range expected.NetworkIDs {
			if result.NetworkIDs[i] != expected.NetworkIDs[i] {
				fmt.Printf("Expected NetworkIDs[%d]: %v, but got: %v\n", i, expected.NetworkIDs[i], result.NetworkIDs[i])
				isEqual = false
			}
		}
	}

	if len(result.SecurityGroups) != len(expected.SecurityGroups) {
		fmt.Printf("Expected SecurityGroups length: %v, but got: %v\n", len(expected.SecurityGroups), len(result.SecurityGroups))
		isEqual = false
	} else {
		for i := range expected.SecurityGroups {
			if result.SecurityGroups[i] != expected.SecurityGroups[i] {
				fmt.Printf("Expected SecurityGroups[%d]: %v, but got: %v\n", i, expected.SecurityGroups[i], result.SecurityGroups[i])
				isEqual = false
			}
		}
	}
	if result.FloatingIpNetworkID != expected.FloatingIpNetworkID {
		fmt.Printf("Expected FloatingIpNetworkID: %v, but got: %v\n", expected.FloatingIpNetworkID, result.FloatingIpNetworkID)
		isEqual = false
	}
	if result.Username != expected.Username {
		fmt.Printf("Expected Username: %v, but got: %v\n", expected.Username, result.Username)
		isEqual = false
	}
	if result.Password != expected.Password {
		fmt.Printf("Expected Password: %v, but got: %v\n", expected.Password, result.Password)
		isEqual = false
	}
	if result.Region != expected.Region {
		fmt.Printf("Expected Region: %v, but got: %v\n", expected.Region, result.Region)
		isEqual = false
	}
	if result.TenantName != expected.TenantName {
		fmt.Printf("Expected TenantName: %v, but got: %v\n", expected.TenantName, result.TenantName)
		isEqual = false
	}
	if result.DomainName != expected.DomainName {
		fmt.Printf("Expected DomainName: %v, but got: %v\n", expected.DomainName, result.DomainName)
		isEqual = false
	}
	if result.IdentityEndpoint != expected.IdentityEndpoint {
		fmt.Printf("Expected IdentityEndpoint: %v, but got: %v\n", expected.IdentityEndpoint, result.IdentityEndpoint)
		isEqual = false
	}

	return isEqual
}

func TestLoadEnv(t *testing.T) {
	testManager := &Manager{}
	testManager.LoadEnv()
}

func TestManagerNewProvider(t *testing.T) {
	server := CreateServer()

	tests := []struct {
		name    string
		input   Config
		wantErr bool
	}{
		{
			name: "ValidConfig",
			input: Config{
				IdentityEndpoint:    server.URL + "/v3",
				Username:            "test-user",
				TenantName:          "test-tenant",
				Password:            "test-password",
				DomainName:          "test-domain",
				Region:              "test-region",
				ServerPrefix:        "test-vm-name",
				ImageID:             "test-image-id",
				FlavorID:            "test-flavor-id",
				NetworkIDs:          []string{"net-1"},
				SecurityGroups:      []string{"sg-1"},
				FloatingIpNetworkID: "floating-net",
			},
			wantErr: false,
		},
		{
			name: "InvalidEndpointConfig",
			input: Config{
				IdentityEndpoint:    "http://bad-address.example.com/v3",
				Username:            "test-user",
				TenantName:          "test-tenant",
				Password:            "test-password",
				DomainName:          "test-domain",
				Region:              "test-region",
				ServerPrefix:        "test-vm-name",
				ImageID:             "test-image-id",
				FlavorID:            "test-flavor-id",
				NetworkIDs:          []string{"net-1"},
				SecurityGroups:      []string{"sg-1"},
				FloatingIpNetworkID: "floating-net",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openstackcfg = tt.input

			testManager := &Manager{}
			provider, err := testManager.NewProvider()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
				if provider != nil {
					t.Errorf("Expected provider to be nil for test case %v, but got: %v", tt.name, provider)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				}
				if provider != nil {
					t.Logf("Provider successfully created: %v", tt.name)
				} else {
					t.Errorf("Expected provider to be created for test case %v, but got nil", tt.name)
				}
			}
			openstackcfg = Config{}
		})
	}
}

func TestGetConfig(t *testing.T) {

	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	testManager := &Manager{}

	args := []string{
		"-server-prefix=test-vm-name",
		"-imageID=test-image-id",
		"-flavorID=test-flavor-id",
		"-networkID=net-1",
		"-security-group=sg-1",
		"-floating-ip-networkID=floating-net",
	}

	os.Setenv("OPENSTACK_USERNAME", "test-user")
	os.Setenv("OPENSTACK_PASSWORD", "test-password")
	os.Setenv("OPENSTACK_REGION", "test-region")
	os.Setenv("OPENSTACK_TENANT_NAME", "test-tenant")
	os.Setenv("OPENSTACK_DOMAIN_NAME", "test-domain")
	os.Setenv("OPENSTACK_IDENTITY_ENDPOINT", "https://identity.testopenstack/v3")

	testManager.ParseCmd(flags)
	err := flags.Parse(args)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	expectedConfig := Config{
		ServerPrefix:        "test-vm-name",
		ImageID:             "test-image-id",
		FlavorID:            "test-flavor-id",
		NetworkIDs:          []string{"net-1"},
		SecurityGroups:      []string{"sg-1"},
		FloatingIpNetworkID: "floating-net",
		Username:            "test-user",
		Password:            "test-password",
		Region:              "test-region",
		TenantName:          "test-tenant",
		DomainName:          "test-domain",
		IdentityEndpoint:    "https://identity.testopenstack/v3",
	}

	testManager.LoadEnv()
	testManager.GetConfig()

	if !comparestructs(openstackcfg, expectedConfig) {
		t.Errorf("After LoadEnv: Expected config: %+v, but got: %+v", expectedConfig, openstackcfg)
	} else {
		t.Logf("Expected config: %+v, got config: %+v\n", expectedConfig, openstackcfg)
	}

	os.Unsetenv("OPENSTACK_USERNAME")
	os.Unsetenv("OPENSTACK_PASSWORD")
	os.Unsetenv("OPENSTACK_REGION")
	os.Unsetenv("OPENSTACK_TENANT_NAME")
	os.Unsetenv("OPENSTACK_DOMAIN_NAME")
	os.Unsetenv("OPENSTACK_IDENTITY_ENDPOINT")
	openstackcfg = Config{}
}
