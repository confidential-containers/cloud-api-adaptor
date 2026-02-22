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
