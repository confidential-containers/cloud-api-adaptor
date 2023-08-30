// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"fmt"
	"testing"

	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/stretchr/testify/assert"
	libvirtxml "libvirt.org/go/libvirtxml"
)

var testCfg Config

func init() {
	cloud.DefaultToEnv(&testCfg.URI, "LIBVIRT_URI", "") // explicitly no fallback here
	cloud.DefaultToEnv(&testCfg.PoolName, "LIBVIRT_POOL", defaultPoolName)
	cloud.DefaultToEnv(&testCfg.NetworkName, "LIBVIRT_NET", defaultNetworkName)
	cloud.DefaultToEnv(&testCfg.VolName, "LIBVIRT_VOL_NAME", defaultVolName)
	cloud.DefaultToEnv(&testCfg.LaunchSecurity, "LIBVIRT_LAUNCH_SECURITY", defaultLaunchSecurity)
	cloud.DefaultToEnv(&testCfg.Firmware, "LIBVIRT_FIRMWARE", defaultFirmware)
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

func TestGetLaunchSecurity(t *testing.T) {
	launchSecurity, err := GetLaunchSecurityType("qemu:///system")
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", launchSecurity.String())
}

func verifyVirtioIOMMU(domXML *libvirtxml.Domain) error {
	// verify we have iommu on the disks
	for i, disk := range domXML.Devices.Disks {
		if disk.Driver.IOMMU != "on" {
			return fmt.Errorf("disk [%d] does not have IOMMU assigned", i)
		}
	}
	// verify we have iommu on the networks
	for i, iface := range domXML.Devices.Interfaces {
		if iface.Driver.IOMMU != "on" {
			return fmt.Errorf("interface [%d] does not have IOMMU assigned", i)
		}
	}
	return nil
}

func verifyDomainXMLs390x(domXML *libvirtxml.Domain) error {
	err := verifyVirtioIOMMU(domXML)
	if err != nil {
		return err
	}
	return nil
}
func verifyDomainXMLx86_64(domXML *libvirtxml.Domain) error {

	if domXML.LaunchSecurity.SEV != nil {
		return verifySEVSettings(domXML)
	}

	return nil
}

func verifySEVSettings(domXML *libvirtxml.Domain) error {
	if domXML.LaunchSecurity.SEV == nil {
		return fmt.Errorf("Launch Security is not enabled")
	}

	const q35 = "q35"
	const i440fx = "i440fx"

	machine := domXML.OS.Type.Machine
	if !(strings.Contains(machine, q35) || strings.Contains(machine, i440fx)) {
		return fmt.Errorf("Machine does not support machine type %s", machine)
	}
	if !strings.Contains(machine, q35) {
		fmt.Printf("Only q35 machines are recommended for SEV\n")
	}

	// SEV only works on OVMF (UEFI)
	if !strings.Contains(domXML.OS.Loader.Path, "OVMF_CODE") {
		return fmt.Errorf("Boot Loader must be OVMF (UEFI) [%s]", domXML.OS.Loader.Path)
	}

	err := verifyVirtioIOMMU(domXML)
	if err != nil {
		return err
	}

	return nil
}

func verifyDomainXML(domXML *libvirtxml.Domain) error {
	arch := domXML.OS.Type.Arch
	switch arch {
	case ArchS390x:
		return verifyDomainXMLs390x(domXML)
	default:
		return verifyDomainXMLx86_64(domXML)
	}
}

func TestCreateDomainXMLs390x(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	if err != nil {
		t.Error(err)
	}
	defer client.connection.Close()

	domainCfg := domainConfig{
		name:        "TestCreateDomainS390x",
		cpu:         2,
		mem:         2,
		networkName: client.networkName,
		bootDisk:    "/var/lib/libvirt/images/root.qcow2",
		cidataDisk:  "/var/lib/libvirt/images/cidata.iso",
	}

	vm := vmConfig{}

	domCfg, err := createDomainXML(client, &domainCfg, &vm)
	if err != nil {
		t.Error(err)
	}

	arch := domCfg.OS.Type.Arch
	if domCfg.OS.Type.Arch != ArchS390x {
		t.Skipf("Skipping because architecture is [%s] and not [%s].", arch, ArchS390x)
	}

	// verify the config
	err = verifyDomainXML(domCfg)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateDomainXMLSEV(t *testing.T) {
	checkConfig(t)

	client, err := NewLibvirtClient(testCfg)
	if err != nil {
		t.Error(err)
	}
	defer client.connection.Close()

	vm := vmConfig{
		name:               "TestCreateDomainS390x",
		cpu:                2,
		mem:                2,
		launchSecurityType: SEV,
	}

	domainCfg := domainConfig{
		name:        vm.name,
		cpu:         vm.cpu,
		mem:         vm.mem,
		networkName: client.networkName,
		bootDisk:    "/var/lib/libvirt/images/root.qcow2",
		cidataDisk:  "/var/lib/libvirt/images/cidata.iso",
	}

	arch := client.nodeInfo.Model
	if arch != "x86_64" {
		t.Skipf("SEV is supported on q35 machines which run x86_64, not %s", arch)
	}
	guest, err := getGuestForArchType(client.caps, arch, "hvm")
	if err != nil {
		t.Skipf("unable to find guest machine to determine SEV capabilities")
	}

	var domCapflags uint32 = 0
	domCaps, err := GetDomainCapabilities(client.connection, guest.Arch.Emulator, arch, "q35", "qemu", domCapflags)
	if err != nil {
		t.Skipf("unable to determine guest domain capabilities: %+v", err)
	}
	if domCaps.Features.SEV.Supported != "yes" {
		t.Skipf("SEV is not supported for this domain")
	}

	domCfg, err := createDomainXML(client, &domainCfg, &vm)
	if err != nil {
		t.Error(err)
	}

	// verify the config
	err = verifyDomainXML(domCfg)
	if err != nil {
		t.Error(err)
	}
}
