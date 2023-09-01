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
	checkConfig(t)

	launchSecurity, err := GetLaunchSecurityType(testCfg.URI)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", launchSecurity.String())
}

func TestGetSEVGuestPolicy(t *testing.T) {
	testPolicy0 := sevGuestPolicy{
		noDebug:    false,
		noKeyShare: false,
		es:         false,
		noSend:     false,
		domain:     false,
		sev:        false,
	}
	uintPolicy := testPolicy0.getGuestPolicy()
	if uintPolicy != 0 {
		t.Errorf("Expected 0 got %d", uintPolicy)
	}

	testPolicy1 := sevGuestPolicy{
		noDebug:    true,
		noKeyShare: true,
		es:         true,
		noSend:     true,
		domain:     true,
		sev:        true,
	}
	uintPolicy = testPolicy1.getGuestPolicy()
	if uintPolicy != 63 {
		t.Errorf("Expected 63 got %d", uintPolicy)
	}

	testPolicy2 := sevGuestPolicy{
		noDebug:    true,
		noKeyShare: false,
		es:         true,
		noSend:     false,
		domain:     true,
		sev:        false,
	}
	uintPolicy = testPolicy2.getGuestPolicy()
	if uintPolicy != 21 {
		t.Errorf("Expected 21 got %d", uintPolicy)
	}

	testPolicy3 := sevGuestPolicy{
		noDebug:    false,
		noKeyShare: true,
		es:         false,
		noSend:     true,
		domain:     false,
		sev:        true,
	}
	uintPolicy = testPolicy3.getGuestPolicy()
	if uintPolicy != 42 {
		t.Errorf("Expected 42 got %d", uintPolicy)
	}

}

func verifyDomainXMLs390x(domXML *libvirtxml.Domain) error {
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
		return fmt.Errorf("machine does not support machine type %s", machine)
	}
	if !strings.Contains(machine, q35) {
		fmt.Printf("only q35 machines are recommended for SEV\n")
	}

	// SEV only works on OVMF (UEFI)
	if !strings.Contains(domXML.OS.Loader.Path, "OVMF_CODE") {
		return fmt.Errorf("boot Loader must be OVMF (UEFI) [%s]", domXML.OS.Loader.Path)
	}

	// verify all virtio devices have IOMMU enabled
	for devControllerNum := range domXML.Devices.Controllers {
		if strings.Contains(domXML.Devices.Controllers[devControllerNum].Type, "virtio") ||
			strings.Contains(domXML.Devices.Controllers[devControllerNum].Model, "virtio") {
			if domXML.Devices.Controllers[devControllerNum].Driver.IOMMU != "on" {
				return fmt.Errorf("virtio controllers must have IOMMU enabled")
			}
		}
	}
	for devInterfaceNum := range domXML.Devices.Interfaces {
		if domXML.Devices.Interfaces[devInterfaceNum].Model.Type == "virtio" {
			if domXML.Devices.Interfaces[devInterfaceNum].Driver.IOMMU != "on" {
				return fmt.Errorf("virtio interfaces must have IOMMU enabled")
			}
			if domXML.Devices.Interfaces[devInterfaceNum].Source.Network != nil {
				if domXML.Devices.Interfaces[devInterfaceNum].ROM.Enabled != "no" {
					return fmt.Errorf("virtio-net must have ROM option disabled")
				}
			}
		}
	}
	for devInputNum := range domXML.Devices.Inputs {
		if domXML.Devices.Inputs[devInputNum].Bus == "virtio" {
			if domXML.Devices.Inputs[devInputNum].Driver != nil {
				if domXML.Devices.Inputs[devInputNum].Driver.IOMMU != "on" {
					return fmt.Errorf("virtio input devices must have IOMMU enabled")
				}
			}
		}
	}
	for devVideoNum := range domXML.Devices.Videos {
		if domXML.Devices.Videos[devVideoNum].Model.Type == "virtio" {
			if domXML.Devices.Videos[devVideoNum].Driver != nil {
				if domXML.Devices.Videos[devVideoNum].Driver.IOMMU != "on" {
					return fmt.Errorf("virtio video devices must have IOMMU enabled")
				}
			}
		}
	}
	if domXML.Devices.MemBalloon != nil && strings.Contains(domXML.Devices.MemBalloon.Model, "virtio") {
		if domXML.Devices.MemBalloon.Driver == nil || domXML.Devices.MemBalloon.Driver.IOMMU != "on" {
			return fmt.Errorf("virtio memballoon must have IOMMU enabled")
		}
	}
	for devRNGNum := range domXML.Devices.RNGs {
		if strings.Contains(domXML.Devices.RNGs[devRNGNum].Model, "virtio") {
			if domXML.Devices.RNGs[devRNGNum].Driver.IOMMU != "on" {
				return fmt.Errorf("virtio rng device must have IOMMU enabled")
			}
		}
	}
	if domXML.Devices.VSock != nil && strings.Contains(domXML.Devices.VSock.Model, "virtio") {
		if domXML.Devices.VSock.Driver.IOMMU != "on" {
			return fmt.Errorf("virtio vsock device must have IOMMU enabled")
		}
	}

	return nil
}

func verifyDomainXML(domXML *libvirtxml.Domain) error {
	arch := domXML.OS.Type.Arch
	switch arch {
	case archS390x:
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
		firmware:           "/usr/share/edk2/ovmf/OVMF_CODE.fd",
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
