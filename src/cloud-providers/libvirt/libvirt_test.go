// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"fmt"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	libvirt "libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
)

const (
	testBootDisk     = "/var/lib/libvirt/images/root.qcow2"
	testCiDataISO    = "/var/lib/libvirt/images/cidata.iso"
	testCloudInitISO = "/var/lib/libvirt/images/cloudinit.iso"
	testNetworkName  = "default"
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
	require.NoError(t, err)
	defer client.connection.Close()

	assert.NotEmpty(t, client.nodeInfo)
	assert.NotEmpty(t, client.caps)
}

func TestGetArchitecture(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	require.NoError(t, err)
	defer client.connection.Close()

	node, err := client.connection.GetNodeInfo()
	require.NoError(t, err)

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

func createTestDomainConfig(name string, cpu, mem uint, networkName, cidataDisk string) *domainConfig {
	return &domainConfig{
		name:        name,
		cpu:         cpu,
		mem:         mem,
		networkName: networkName,
		bootDisk:    testBootDisk,
		cidataDisk:  cidataDisk,
	}
}

func TestCreateDomainXMLArchitectures(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	require.NoError(t, err)
	defer client.connection.Close()

	tests := []struct {
		name         string
		domainName   string
		cpu          uint
		mem          uint
		cidataDisk   string
		expectedArch string
	}{
		{
			name:         "s390x architecture",
			domainName:   "TestCreateDomainS390x",
			cpu:          2,
			mem:          2,
			cidataDisk:   testCiDataISO,
			expectedArch: archS390x,
		},
		{
			name:         "aarch64 architecture",
			domainName:   "TestCreateDomainAArch64",
			cpu:          2,
			mem:          4,
			cidataDisk:   testCloudInitISO,
			expectedArch: archAArch64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := vmConfig{}

			domainCfg := domainConfig{
				name:        tt.domainName,
				cpu:         tt.cpu,
				mem:         tt.mem,
				networkName: client.networkName,
				bootDisk:    testBootDisk,
				cidataDisk:  tt.cidataDisk,
			}

			domCfg, err := createDomainXML(client, &domainCfg, &vm)
			assert.NoError(t, err)

			arch := domCfg.OS.Type.Arch
			if domCfg.OS.Type.Arch != tt.expectedArch {
				t.Skipf("Skipping because architecture is [%s] and not [%s].", arch, tt.expectedArch)
			}

			// verify the config
			err = verifyDomainXML(domCfg)
			assert.NoError(t, err)
		})
	}
}

func TestGetDeletableDiskPaths(t *testing.T) {
	tests := []struct {
		name     string
		domain   *libvirtxml.Domain
		expected []string
	}{
		{
			name: "extract file-backed disks only",
			domain: &libvirtxml.Domain{
				Devices: &libvirtxml.DomainDeviceList{
					Disks: []libvirtxml.DomainDisk{
						{
							Source: &libvirtxml.DomainDiskSource{
								File: &libvirtxml.DomainDiskSourceFile{File: testBootDisk},
							},
						},
						{
							Source: &libvirtxml.DomainDiskSource{
								File: &libvirtxml.DomainDiskSourceFile{File: testCloudInitISO},
							},
						},
					},
				},
			},
			expected: []string{
				testBootDisk,
				testCloudInitISO,
			},
		},
		{
			name: "skip disks without file source",
			domain: &libvirtxml.Domain{
				Devices: &libvirtxml.DomainDeviceList{
					Disks: []libvirtxml.DomainDisk{
						{
							Source: &libvirtxml.DomainDiskSource{
								File: &libvirtxml.DomainDiskSourceFile{File: testBootDisk},
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
				testBootDisk,
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
				Devices: &libvirtxml.DomainDeviceList{
					Disks: []libvirtxml.DomainDisk{},
				},
			},
			expected: []string{},
		},
		{
			name: "skip disks with empty file path",
			domain: &libvirtxml.Domain{
				Devices: &libvirtxml.DomainDeviceList{
					Disks: []libvirtxml.DomainDisk{
						{
							Source: &libvirtxml.DomainDiskSource{
								File: &libvirtxml.DomainDiskSourceFile{File: ""},
							},
						},
					},
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := getDeletableDiskPaths(tt.domain)
			assert.Equal(t, tt.expected, paths)
		})
	}
}

func TestGetGuestForArchType(t *testing.T) {
	tests := []struct {
		name        string
		caps        *libvirtxml.Caps
		arch        string
		ostype      string
		expectError bool
		expectArch  string
	}{
		{
			name:        "find x86_64 guest",
			caps:        createMockCaps("x86_64", ""),
			arch:        "x86_64",
			ostype:      "hvm",
			expectError: false,
			expectArch:  "x86_64",
		},
		{
			name:        "find s390x guest",
			caps:        createMockCaps("s390x", ""),
			arch:        "s390x",
			ostype:      "hvm",
			expectError: false,
			expectArch:  "s390x",
		},
		{
			name:        "find aarch64 guest",
			caps:        createMockCaps("aarch64", ""),
			arch:        "aarch64",
			ostype:      "hvm",
			expectError: false,
			expectArch:  "aarch64",
		},
		{
			name:        "architecture not found",
			caps:        createMockCaps("x86_64", ""),
			arch:        "invalid-arch",
			ostype:      "hvm",
			expectError: true,
		},
		{
			name: "ostype mismatch",
			caps: &libvirtxml.Caps{
				Guests: []libvirtxml.CapsGuest{
					{
						OSType: "xen",
						Arch: libvirtxml.CapsGuestArch{
							Name: "x86_64",
						},
					},
				},
			},
			arch:        "x86_64",
			ostype:      "hvm",
			expectError: true,
		},
		{
			name: "empty capabilities",
			caps: &libvirtxml.Caps{
				Guests: []libvirtxml.CapsGuest{},
			},
			arch:        "x86_64",
			ostype:      "hvm",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guest, err := getGuestForArchType(tt.caps, tt.arch, tt.ostype)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, guest)
				assert.ErrorContains(t, err, "could not find any guests")
			} else {
				assert.NoError(t, err)
				require.NotNil(t, guest)
				assert.Equal(t, tt.expectArch, guest.Arch.Name)
				assert.Equal(t, tt.ostype, guest.OSType)
			}
		})
	}
}

func TestLookupMachine(t *testing.T) {
	tests := []struct {
		name           string
		machines       []libvirtxml.CapsGuestMachine
		targetMachine  string
		expectedResult string
	}{
		{
			name: "find machine with canonical name",
			machines: []libvirtxml.CapsGuestMachine{
				{Name: "pc", Canonical: "pc-i440fx-2.12"},
				{Name: "q35", Canonical: "pc-q35-2.12"},
			},
			targetMachine:  "pc",
			expectedResult: "pc-i440fx-2.12",
		},
		{
			name: "find machine without canonical name",
			machines: []libvirtxml.CapsGuestMachine{
				{Name: "virt"},
				{Name: "pc"},
			},
			targetMachine:  "virt",
			expectedResult: "virt",
		},
		{
			name: "machine not found returns empty string",
			machines: []libvirtxml.CapsGuestMachine{
				{Name: "pc"},
				{Name: "q35"},
			},
			targetMachine:  "nonexistent",
			expectedResult: "",
		},
		{
			name: "s390x machine with canonical",
			machines: []libvirtxml.CapsGuestMachine{
				{Name: "s390-ccw-virtio", Canonical: "s390-ccw-virtio-rhel9.0.0"},
			},
			targetMachine:  "s390-ccw-virtio",
			expectedResult: "s390-ccw-virtio-rhel9.0.0",
		},
		{
			name:           "empty machine list",
			machines:       []libvirtxml.CapsGuestMachine{},
			targetMachine:  "pc",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lookupMachine(tt.machines, tt.targetMachine)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetCanonicalMachineName(t *testing.T) {
	tests := []struct {
		name           string
		caps           *libvirtxml.Caps
		arch           string
		virttype       string
		targetMachine  string
		expectedResult string
		expectError    bool
	}{
		{
			name: "find canonical machine in arch machines",
			caps: createMockCaps("x86_64", "",
				libvirtxml.CapsGuestMachine{Name: "pc", Canonical: "pc-i440fx-2.12"}),
			arch:           "x86_64",
			virttype:       "hvm",
			targetMachine:  "pc",
			expectedResult: "pc-i440fx-2.12",
			expectError:    false,
		},
		{
			name: "find canonical machine in domain machines",
			caps: &libvirtxml.Caps{
				Guests: []libvirtxml.CapsGuest{
					{
						OSType: "hvm",
						Arch: libvirtxml.CapsGuestArch{
							Name: "x86_64",
							Domains: []libvirtxml.CapsGuestDomain{
								{
									Machines: []libvirtxml.CapsGuestMachine{
										{Name: "q35", Canonical: "pc-q35-2.12"},
									},
								},
							},
						},
					},
				},
			},
			arch:           "x86_64",
			virttype:       "hvm",
			targetMachine:  "q35",
			expectedResult: "pc-q35-2.12",
			expectError:    false,
		},
		{
			name: "machine not found returns error",
			caps: createMockCaps("x86_64", "",
				libvirtxml.CapsGuestMachine{Name: "pc"}),
			arch:          "x86_64",
			virttype:      "hvm",
			targetMachine: "nonexistent",
			expectError:   true,
		},
		{
			name:          "architecture not found returns error",
			caps:          createMockCaps("x86_64", ""),
			arch:          "invalid-arch",
			virttype:      "hvm",
			targetMachine: "pc",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getCanonicalMachineName(tt.caps, tt.arch, tt.virttype, tt.targetMachine)
			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestCreateCloudInitISO(t *testing.T) {
	tests := []struct {
		name        string
		vmConfig    *vmConfig
		expectError bool
	}{
		{
			name: "valid cloud-init data",
			vmConfig: &vmConfig{
				name:     "test-vm",
				userData: "#cloud-config\nruncmd:\n  - echo 'Hello World'\n",
			},
			expectError: false,
		},
		{
			name: "empty user data",
			vmConfig: &vmConfig{
				name:     "test-vm",
				userData: "",
			},
			expectError: false,
		},
		{
			name: "large user data",
			vmConfig: &vmConfig{
				name:     "test-vm",
				userData: "#cloud-config\n" + string(make([]byte, 10000)),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isoData, err := createCloudInitISO(tt.vmConfig)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, isoData)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, isoData)
			}
		})
	}
}

func TestConfigVerifier(t *testing.T) {
	newConfig := func(overrides func(*Config)) *Config {
		cfg := &Config{
			URI:         "qemu:///system",
			PoolName:    "default",
			NetworkName: testNetworkName,
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

			assert.EqualError(t, err, tc.expectedError)
		})
	}
}

func TestGetLaunchSecurityTypeInvalidURI(t *testing.T) {
	_, err := GetLaunchSecurityType("invalid://uri")
	assert.Error(t, err)
}

func createMockCaps(arch, emulator string, machines ...libvirtxml.CapsGuestMachine) *libvirtxml.Caps {
	return &libvirtxml.Caps{
		Guests: []libvirtxml.CapsGuest{
			{
				OSType: "hvm",
				Arch: libvirtxml.CapsGuestArch{
					Name:     arch,
					Emulator: emulator,
					Machines: machines,
				},
			},
		},
	}
}

// TestCreateDomainXMLs390xWithMocks tests s390x domain XML generation using mocks
func TestCreateDomainXMLArchitecturesWithMocks(t *testing.T) {
	tests := []struct {
		name                 string
		arch                 string
		vmName               string
		cpu                  uint
		mem                  uint
		cidataDisk           string
		emulator             string
		machineName          string
		machineCanonical     string
		createFunc           func(*libvirtClient, *domainConfig, *vmConfig) (*libvirtxml.Domain, error)
		expectedFirmware     string
		expectSCSIController bool
	}{
		{
			name:             "s390x architecture",
			arch:             "s390x",
			vmName:           "test-s390x-vm",
			cpu:              2,
			mem:              2048,
			cidataDisk:       testCiDataISO,
			emulator:         "/usr/bin/qemu-system-s390x",
			machineName:      "s390-ccw-virtio",
			machineCanonical: "s390-ccw-virtio-rhel9.0.0",
			createFunc:       createDomainXMLs390x,
		},
		{
			name:                 "aarch64 architecture",
			arch:                 "aarch64",
			vmName:               "test-aarch64-vm",
			cpu:                  4,
			mem:                  4096,
			cidataDisk:           testCloudInitISO,
			emulator:             "/usr/bin/qemu-system-aarch64",
			machineName:          "virt",
			machineCanonical:     "virt-4.2",
			createFunc:           createDomainXMLaarch64,
			expectedFirmware:     "efi",
			expectSCSIController: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock capabilities
			mockCaps := createMockCaps(tt.arch, tt.emulator, libvirtxml.CapsGuestMachine{
				Name:      tt.machineName,
				Canonical: tt.machineCanonical,
			})

			mockClient := &libvirtClient{
				caps:        mockCaps,
				networkName: testNetworkName,
			}

			cfg := createTestDomainConfig(tt.vmName, tt.cpu, tt.mem, testNetworkName, tt.cidataDisk)

			vm := &vmConfig{}

			domain, err := tt.createFunc(mockClient, cfg, vm)
			assert.NoError(t, err)
			require.NotNil(t, domain)

			// Verify domain properties
			assert.Equal(t, "kvm", domain.Type)
			assert.Equal(t, tt.vmName, domain.Name)
			assert.Equal(t, tt.cpu, domain.VCPU.Value)
			assert.Equal(t, tt.mem, domain.Memory.Value)
			assert.Equal(t, tt.arch, domain.OS.Type.Arch)
			assert.Equal(t, tt.machineCanonical, domain.OS.Type.Machine)

			if tt.expectedFirmware != "" {
				assert.Equal(t, tt.expectedFirmware, domain.OS.Firmware)
			}

			// Verify disks
			assert.Len(t, domain.Devices.Disks, 2)
			assert.Equal(t, testBootDisk, domain.Devices.Disks[0].Source.File.File)
			assert.Equal(t, "on", domain.Devices.Disks[0].Driver.IOMMU)
			assert.Equal(t, tt.cidataDisk, domain.Devices.Disks[1].Source.File.File)

			// Verify network interface has IOMMU
			assert.Len(t, domain.Devices.Interfaces, 1)
			assert.Equal(t, "on", domain.Devices.Interfaces[0].Driver.IOMMU)

			// Verify SCSI controller if expected
			if tt.expectSCSIController {
				assert.Len(t, domain.Devices.Controllers, 1)
				assert.Equal(t, "scsi", domain.Devices.Controllers[0].Type)
			}
		})
	}
}

// TestCreateDomainXMLx86_64 tests x86_64 domain XML generation
func TestCreateDomainXMLx86_64(t *testing.T) {
	mockCaps := createMockCaps("x86_64", "/usr/bin/qemu-system-x86_64")

	mockClient := &libvirtClient{
		caps:        mockCaps,
		networkName: testNetworkName,
	}

	tests := []struct {
		name          string
		cfg           *domainConfig
		vm            *vmConfig
		expectError   bool
		checkFirmware bool
		expectedFW    string
	}{
		{
			name: "basic x86_64 domain",
			cfg:  createTestDomainConfig("test-x86-vm", 2, 2048, testNetworkName, testCiDataISO),
			vm: &vmConfig{
				launchSecurityType: NoLaunchSecurity,
			},
			expectError: false,
		},
		{
			name: "x86_64 with firmware",
			cfg:  createTestDomainConfig("test-x86-fw-vm", 4, 4096, testNetworkName, testCiDataISO),
			vm: &vmConfig{
				launchSecurityType: NoLaunchSecurity,
				firmware:           "/usr/share/OVMF/OVMF_CODE.fd",
			},
			expectError:   false,
			checkFirmware: true,
			expectedFW:    "efi",
		},
		{
			name: "unsupported security type",
			cfg:  createTestDomainConfig("test-x86-sec-vm", 2, 2048, testNetworkName, testCiDataISO),
			vm: &vmConfig{
				launchSecurityType: S390PV, // Not supported on x86_64
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := createDomainXMLx86_64(mockClient, tt.cfg, tt.vm)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, domain)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, domain)

			// Verify basic properties
			assert.Equal(t, "kvm", domain.Type)
			assert.Equal(t, tt.cfg.name, domain.Name)
			assert.Equal(t, tt.cfg.cpu, domain.VCPU.Value)
			assert.Equal(t, tt.cfg.mem, domain.Memory.Value)
			assert.Equal(t, "x86_64", domain.OS.Type.Arch)

			// Verify disks
			assert.Len(t, domain.Devices.Disks, 2)
			assert.Equal(t, tt.cfg.bootDisk, domain.Devices.Disks[0].Source.File.File)
			assert.Equal(t, tt.cfg.cidataDisk, domain.Devices.Disks[1].Source.File.File)

			// Check firmware if specified
			if tt.checkFirmware {
				assert.NotEmpty(t, domain.OS.Loader)
				assert.Equal(t, tt.vm.firmware, domain.OS.Loader.Path)
				assert.Equal(t, tt.expectedFW, domain.OS.Firmware)
			}
		})
	}
}

// TestCreateDomainXML tests the architecture-based domain XML dispatcher
func TestCreateDomainXML(t *testing.T) {
	tests := []struct {
		name         string
		arch         string
		expectedArch string
	}{
		{
			name:         "s390x architecture",
			arch:         "s390x",
			expectedArch: "s390x",
		},
		{
			name:         "aarch64 architecture",
			arch:         "aarch64",
			expectedArch: "aarch64",
		},
		{
			name:         "x86_64 architecture (default)",
			arch:         "x86_64",
			expectedArch: "x86_64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create appropriate mock capabilities using helper function
			var mockCaps *libvirtxml.Caps
			switch tt.arch {
			case "s390x":
				mockCaps = createMockCaps("s390x", "/usr/bin/qemu-system-s390x",
					libvirtxml.CapsGuestMachine{Name: "s390-ccw-virtio", Canonical: "s390-ccw-virtio-rhel9.0.0"})
			case "aarch64":
				mockCaps = createMockCaps("aarch64", "/usr/bin/qemu-system-aarch64",
					libvirtxml.CapsGuestMachine{Name: "virt", Canonical: "virt-4.2"})
			default:
				mockCaps = createMockCaps("x86_64", "/usr/bin/qemu-system-x86_64")
			}

			mockClient := &libvirtClient{
				caps:        mockCaps,
				networkName: testNetworkName,
				nodeInfo:    &libvirt.NodeInfo{Model: tt.arch},
			}

			cfg := createTestDomainConfig("test-vm", 2, 2048, testNetworkName, testCiDataISO)

			vm := &vmConfig{
				launchSecurityType: NoLaunchSecurity,
			}

			domain, err := createDomainXML(mockClient, cfg, vm)
			assert.NoError(t, err)
			require.NotNil(t, domain)
			assert.Equal(t, tt.expectedArch, domain.OS.Type.Arch)
		})
	}
}

// TestVerifyDomainXMLIOMMU tests IOMMU verification for s390x and aarch64
func TestVerifyDomainXMLIOMMU(t *testing.T) {
	tests := []struct {
		name        string
		domain      *libvirtxml.Domain
		expectError bool
		errorMsg    string
	}{
		{
			name: "s390x with proper IOMMU on disks and interfaces",
			domain: &libvirtxml.Domain{
				OS: &libvirtxml.DomainOS{
					Type: &libvirtxml.DomainOSType{Arch: "s390x"},
				},
				Devices: &libvirtxml.DomainDeviceList{
					Disks: []libvirtxml.DomainDisk{
						{
							Target: &libvirtxml.DomainDiskTarget{Bus: "virtio"},
							Driver: &libvirtxml.DomainDiskDriver{IOMMU: "on"},
						},
					},
					Interfaces: []libvirtxml.DomainInterface{
						{
							Model:  &libvirtxml.DomainInterfaceModel{Type: "virtio"},
							Driver: &libvirtxml.DomainInterfaceDriver{IOMMU: "on"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "s390x missing IOMMU on disk",
			domain: &libvirtxml.Domain{
				OS: &libvirtxml.DomainOS{
					Type: &libvirtxml.DomainOSType{Arch: "s390x"},
				},
				Devices: &libvirtxml.DomainDeviceList{
					Disks: []libvirtxml.DomainDisk{
						{
							Target: &libvirtxml.DomainDiskTarget{Bus: "virtio"},
							Driver: &libvirtxml.DomainDiskDriver{}, // Missing IOMMU
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "does not have IOMMU assigned",
		},
		{
			name: "aarch64 missing IOMMU on interface",
			domain: &libvirtxml.Domain{
				OS: &libvirtxml.DomainOS{
					Type: &libvirtxml.DomainOSType{Arch: "aarch64"},
				},
				Devices: &libvirtxml.DomainDeviceList{
					Interfaces: []libvirtxml.DomainInterface{
						{
							Model:  &libvirtxml.DomainInterfaceModel{Type: "virtio"},
							Driver: &libvirtxml.DomainInterfaceDriver{}, // Missing IOMMU
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "does not have IOMMU assigned",
		},
		{
			name: "x86_64 does not require IOMMU",
			domain: &libvirtxml.Domain{
				OS: &libvirtxml.DomainOS{
					Type: &libvirtxml.DomainOSType{Arch: "x86_64"},
				},
				Devices: &libvirtxml.DomainDeviceList{
					Disks: []libvirtxml.DomainDisk{
						{
							Target: &libvirtxml.DomainDiskTarget{Bus: "virtio"},
							Driver: &libvirtxml.DomainDiskDriver{}, // No IOMMU required
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyDomainXML(tt.domain)
			if tt.expectError {
				assert.ErrorContains(t, err, tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
