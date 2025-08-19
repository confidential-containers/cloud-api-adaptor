//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"flag"
	"strconv"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var libvirtcfg Config

type Manager struct{}

const (
	defaultURI            = "qemu:///system"
	defaultPoolName       = "default"
	defaultNetworkName    = "default"
	defaultDataDir        = "/var/lib/libvirt/images"
	defaultVolName        = "podvm-base.qcow2"
	defaultLaunchSecurity = ""
	defaultFirmware       = ""
	defaultCPU            = "2"
	defaultMemory         = "8192"
)

func init() {
	provider.AddCloudProvider("libvirt", &Manager{})
}

func (*Manager) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&libvirtcfg.URI, "uri", defaultURI, "libvirt URI")
	flags.StringVar(&libvirtcfg.PoolName, "pool-name", defaultPoolName, "libvirt storage pool")
	flags.StringVar(&libvirtcfg.NetworkName, "network-name", defaultNetworkName, "libvirt network pool")
	flags.StringVar(&libvirtcfg.DataDir, "data-dir", defaultDataDir, "libvirt storage dir")
	flags.BoolVar(&libvirtcfg.DisableCVM, "disable-cvm", false, "Use non-CVMs for peer pods")
	flags.StringVar(&libvirtcfg.LaunchSecurity, "launch-security", defaultLaunchSecurity, "Libvirt's LaunchSecurity element for Confidential VMs: s390-pv. If omitted, will automatically determine.")
	flags.StringVar(&libvirtcfg.Firmware, "firmware", defaultFirmware, "Path to OVMF")
	flags.UintVar(&libvirtcfg.CPU, "cpu", 2, "Number of processors allocated")
	flags.UintVar(&libvirtcfg.Memory, "memory", 8192, "Amount of memory in MiB")

}

func (*Manager) LoadEnv() {
	provider.DefaultToEnv(&libvirtcfg.URI, "LIBVIRT_URI", defaultURI)
	provider.DefaultToEnv(&libvirtcfg.PoolName, "LIBVIRT_POOL", defaultPoolName)
	provider.DefaultToEnv(&libvirtcfg.NetworkName, "LIBVIRT_NET", defaultNetworkName)
	provider.DefaultToEnv(&libvirtcfg.VolName, "LIBVIRT_VOL_NAME", defaultVolName)
	provider.DefaultToEnv(&libvirtcfg.LaunchSecurity, "LIBVIRT_LAUNCH_SECURITY", defaultLaunchSecurity)
	provider.DefaultToEnv(&libvirtcfg.Firmware, "LIBVIRT_EFI_FIRMWARE", defaultFirmware)

	var memoryStr, processorsStr string
	provider.DefaultToEnv(&memoryStr, "LIBVIRT_MEMORY", defaultMemory)
	if memoryStr != "" {
		memory, _ := strconv.ParseUint(memoryStr, 10, 32)
		libvirtcfg.Memory = uint(memory)
	}

	provider.DefaultToEnv(&processorsStr, "LIBVIRT_CPU", defaultCPU)
	if processorsStr != "" {
		cpu, _ := strconv.ParseUint(processorsStr, 10, 32)
		libvirtcfg.CPU = uint(cpu)
	}

}

func (*Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&libvirtcfg)
}

func (*Manager) GetConfig() (config *Config) {
	return &libvirtcfg
}
