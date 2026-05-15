// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"fmt"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/stretchr/testify/assert"
	libvirtxml "libvirt.org/go/libvirtxml"
)

var testCfg Config

func init() {
	provider.DefaultToEnv(&testCfg.URI, "LIBVIRT_URI", "") // explicitly no fallback here
	provider.DefaultToEnv(&testCfg.PoolName, "LIBVIRT_POOL", defaultPoolName)
	provider.DefaultToEnv(&testCfg.NetworkName, "LIBVIRT_NET", defaultNetworkName)
	provider.DefaultToEnv(&testCfg.VolName, "LIBVIRT_VOL_NAME", defaultVolName)
}

func checkConfig(t *testing.T) {
	if testCfg.URI == "" {
		t.Skipf("Skipping because LIBVIRT_URI is not configured")
	}
}

func TestLibvirtConnection(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	if err != nil {
		t.Error(err)
	}
	defer client.connection.Close()

	assert.NotNil(t, client.nodeInfo)
	assert.NotNil(t, client.caps)
}

func TestGetArchitecture(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	if err != nil {
		t.Error(err)
	}
	defer client.connection.Close()

	node, err := client.connection.GetNodeInfo()
	if err != nil {
		t.Error(err)
	}

	arch := node.Model
	if arch == "" {
		t.FailNow()
	}
}

func verifyDomainXML(domXML *libvirtxml.Domain) error {
	arch := domXML.OS.Type.Arch
	if arch != archS390x && arch != archAArch64 {
		return nil
	}
	// verify we have iommu on the disks
	for i, disk := range domXML.Devices.Disks {
		if disk.Target.Bus == "virtio" && disk.Driver.IOMMU != "on" {
			return fmt.Errorf("disk [%d] does not have IOMMU assigned", i)
		}
	}
	// verify we have iommu on the networks
	for i, iface := range domXML.Devices.Interfaces {
		if iface.Model.Type == "virtio" && iface.Driver.IOMMU != "on" {
			return fmt.Errorf("interface [%d] does not have IOMMU assigned", i)
		}
	}
	return nil
}

func TestCreateDomainXMLs390x(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	if err != nil {
		t.Error(err)
	}
	defer client.connection.Close()

	vm := vmConfig{}

	domainCfg := domainConfig{
		name:        "TestCreateDomainS390x",
		cpu:         2,
		mem:         2,
		networkName: client.networkName,
		bootDisk:    "/var/lib/libvirt/images/root.qcow2",
		cidataDisk:  "/var/lib/libvirt/images/cidata.iso",
	}

	domCfg, err := createDomainXML(client, &domainCfg, &vm)
	if err != nil {
		t.Error(err)
	}

	arch := domCfg.OS.Type.Arch
	if domCfg.OS.Type.Arch != archS390x {
		t.Skipf("Skipping because architecture is [%s] and not [%s].", arch, archS390x)
	}

	// verify the config
	err = verifyDomainXML(domCfg)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateDomainXMLaarch64(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	if err != nil {
		t.Error(err)
	}
	defer client.connection.Close()

	vm := vmConfig{}

	domainCfg := domainConfig{
		name:        "TestCreateDomainAArch64",
		cpu:         2,
		mem:         4,
		networkName: client.networkName,
		bootDisk:    "/var/lib/libvirt/images/root.qcow2",
		cidataDisk:  "/var/lib/libvirt/images/cloudinit.iso",
	}

	domCfg, err := createDomainXML(client, &domainCfg, &vm)
	if err != nil {
		t.Error(err)
	}

	arch := domCfg.OS.Type.Arch
	if domCfg.OS.Type.Arch != archAArch64 {
		t.Skipf("Skipping because architecture is [%s] and not [%s].", arch, archAArch64)
	}

	err = verifyDomainXML(domCfg)
	if err != nil {
		t.Error(err)
	}
}

func TestGetDeletableDiskPaths(t *testing.T) {
	tests := []struct {
		name     string
		domain   *libvirtxml.Domain
		expected []string
	}{
		{
			name: "returns file-backed disks only",
			domain: &libvirtxml.Domain{
				Devices: &libvirtxml.DomainDeviceList{
					Disks: []libvirtxml.DomainDisk{
						{
							Source: &libvirtxml.DomainDiskSource{
								File: &libvirtxml.DomainDiskSourceFile{File: "/var/lib/libvirt/images/root.qcow2"},
							},
						},
						{
							Source: &libvirtxml.DomainDiskSource{
								File: &libvirtxml.DomainDiskSourceFile{File: "/var/lib/libvirt/images/cloudinit.iso"},
							},
						},
						{
							Source: &libvirtxml.DomainDiskSource{},
						},
						{},
					},
				},
			},
			expected: []string{
				"/var/lib/libvirt/images/root.qcow2",
				"/var/lib/libvirt/images/cloudinit.iso",
			},
		},
		{
			name:     "nil domain returns nil",
			domain:   nil,
			expected: nil,
		},
		{
			name: "empty disk list returns empty slice",
			domain: &libvirtxml.Domain{
				Devices: &libvirtxml.DomainDeviceList{},
			},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			paths := getDeletableDiskPaths(tc.domain)
			assert.Equal(t, tc.expected, paths)
		})
	}
}

func TestConfigVerifier(t *testing.T) {
	newConfig := func(overrides func(*Config)) *Config {
		cfg := &Config{
			URI:         "qemu:///system",
			PoolName:    "default",
			NetworkName: "default",
			VolName:     "podvm-base.qcow2",
			CPU:         1,
			Memory:      512,
		}
		if overrides != nil {
			overrides(cfg)
		}
		return cfg
	}

	tests := []struct {
		name          string
		provider      *libvirtProvider
		expectedError string
	}{
		{
			name: "empty URI fails",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.URI = ""
				}),
			},
			expectedError: "URI is empty",
		},
		{
			name: "empty pool name fails",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.PoolName = ""
				}),
			},
			expectedError: "PoolName is empty",
		},
		{
			name: "empty network name fails",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.NetworkName = ""
				}),
			},
			expectedError: "NetworkName is empty",
		},
		{
			name: "empty volume name fails",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.VolName = ""
				}),
			},
			expectedError: "VolName is empty",
		},
		{
			name: "zero CPU fails",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.CPU = 0
				}),
			},
			expectedError: "CPU must be greater than zero",
		},
		{
			name: "zero memory fails",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.Memory = 0
				}),
			},
			expectedError: "Memory must be greater than zero",
		},
		{
			name: "invalid cpuset fails",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.CPUSet = "ABC"
				}),
			},
			expectedError: "invalid CPUSet format",
		},
		{
			name: "invalid cpuset with mixed alphanumeric fails",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.CPUSet = "0,a,2"
				}),
			},
			expectedError: "invalid CPUSet format",
		},
		{
			name: "valid cpuset passes",
			provider: &libvirtProvider{
				serviceConfig: newConfig(func(c *Config) {
					c.CPUSet = "0,2,4-7"
				}),
			},
			expectedError: "",
		},
		{
			name: "valid config passes",
			provider: &libvirtProvider{
				serviceConfig: newConfig(nil),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.provider.ConfigVerifier()
			if tc.expectedError == "" {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestValidateCPUSet(t *testing.T) {
	testCases := []struct {
		name        string
		cpuset      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid comma-separated",
			cpuset:      "0,2,4,6",
			expectError: false,
		},
		{
			name:        "valid range",
			cpuset:      "0-3",
			expectError: false,
		},
		{
			name:        "valid mixed format",
			cpuset:      "0,2,4-7,10",
			expectError: false,
		},
		{
			name:        "empty cpuset is valid",
			cpuset:      "",
			expectError: false,
		},
		{
			name:        "invalid alphabetic characters",
			cpuset:      "ABC",
			expectError: true,
			errorMsg:    "invalid CPUSet format",
		},
		{
			name:        "invalid mixed alphanumeric",
			cpuset:      "0,a,2",
			expectError: true,
			errorMsg:    "invalid CPUSet format",
		},
		{
			name:        "invalid incomplete range",
			cpuset:      "0-",
			expectError: true,
			errorMsg:    "invalid CPUSet format",
		},
		{
			name:        "invalid leading dash",
			cpuset:      "-3",
			expectError: true,
			errorMsg:    "invalid CPUSet format",
		},
		{
			name:        "invalid special characters",
			cpuset:      "0,2@4",
			expectError: true,
			errorMsg:    "invalid CPUSet format",
		},
		{
			name:        "invalid trailing comma",
			cpuset:      "0,2,",
			expectError: true,
			errorMsg:    "invalid CPUSet format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCPUSet(tc.cpuset)
			if tc.expectError {
				assert.Error(t, err, "Expected error for cpuset: %s", tc.cpuset)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err, "Expected no error for cpuset: %s", tc.cpuset)
			}
		})
	}
}

func TestCPUSetInDomainXML(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.connection.Close()

	testCases := []struct {
		name           string
		cpuset         string
		expectedCPUSet string
	}{
		{
			name:           "with cpuset specified",
			cpuset:         "0,2,4,6",
			expectedCPUSet: "0,2,4,6",
		},
		{
			name:           "with cpuset range",
			cpuset:         "0-3",
			expectedCPUSet: "0-3",
		},
		{
			name:           "with empty cpuset",
			cpuset:         "",
			expectedCPUSet: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vm := vmConfig{
				cpuset: tc.cpuset,
			}

			domainCfg := domainConfig{
				name:        "TestCPUSet",
				cpu:         4,
				mem:         4096,
				networkName: client.networkName,
				bootDisk:    "/var/lib/libvirt/images/root.qcow2",
				cidataDisk:  "/var/lib/libvirt/images/cidata.iso",
			}

			domCfg, err := createDomainXML(client, &domainCfg, &vm)
			if err != nil {
				t.Error(err)
				return
			}

			assert.Equal(t, tc.expectedCPUSet, domCfg.VCPU.CPUSet,
				"CPUSet should be set correctly in domain XML")
		})
	}
}

func TestCPUSetXMLMarshaling(t *testing.T) {
	// Test that CPUSet is correctly marshaled to XML without requiring libvirt connection
	testCases := []struct {
		name           string
		cpuset         string
		expectedSubstr string
	}{
		{
			name:           "cpuset with comma-separated values",
			cpuset:         "0,2,4,6",
			expectedSubstr: `cpuset="0,2,4,6"`,
		},
		{
			name:           "cpuset with range",
			cpuset:         "0-7",
			expectedSubstr: `cpuset="0-7"`,
		},
		{
			name:           "cpuset with mixed format",
			cpuset:         "0,2,4-7,10",
			expectedSubstr: `cpuset="0,2,4-7,10"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domain := &libvirtxml.Domain{
				Type: "kvm",
				Name: "test-vm",
				VCPU: &libvirtxml.DomainVCPU{
					Value:  4,
					CPUSet: tc.cpuset,
				},
			}

			xmlStr, err := domain.Marshal()
			if err != nil {
				t.Errorf("Failed to marshal domain XML: %v", err)
				return
			}

			assert.Contains(t, xmlStr, tc.expectedSubstr,
				"Marshaled XML should contain cpuset attribute")
		})
	}
}
