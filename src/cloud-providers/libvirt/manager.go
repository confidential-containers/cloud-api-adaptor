//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var libvirtcfg Config

type Manager struct{}

const (
	defaultURI            = "qemu+ssh://root@192.168.122.1/system?no_verify=1"
	defaultPoolName       = "default"
	defaultNetworkName    = "default"
	defaultDataDir        = "/var/lib/libvirt/images"
	defaultVolName        = "podvm-base.qcow2"
	defaultLaunchSecurity = ""
	defaultFirmware       = "/usr/share/OVMF/OVMF_CODE_4M.fd"
	defaultCPU            = "2"
	defaultMemory         = "8192"
)

func init() {
	provider.AddCloudProvider("libvirt", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&libvirtcfg.URI, "uri", defaultURI, "LIBVIRT_URI", "libvirt URI")
	reg.StringWithEnv(&libvirtcfg.PoolName, "pool-name", defaultPoolName, "LIBVIRT_POOL", "libvirt storage pool")
	reg.StringWithEnv(&libvirtcfg.NetworkName, "network-name", defaultNetworkName, "LIBVIRT_NET", "libvirt network pool")
	reg.StringWithEnv(&libvirtcfg.VolName, "vol-name", defaultVolName, "LIBVIRT_VOL_NAME", "libvirt volume name")
	reg.StringWithEnv(&libvirtcfg.LaunchSecurity, "launch-security", defaultLaunchSecurity, "LIBVIRT_LAUNCH_SECURITY", "Libvirt's LaunchSecurity element for Confidential VMs: s390-pv. If omitted, will automatically determine.")
	reg.StringWithEnv(&libvirtcfg.Firmware, "firmware", defaultFirmware, "LIBVIRT_EFI_FIRMWARE", "Path to OVMF")
	reg.UintWithEnv(&libvirtcfg.CPU, "cpu", 2, "LIBVIRT_CPU", "Number of processors allocated")
	reg.UintWithEnv(&libvirtcfg.Memory, "memory", 8192, "LIBVIRT_MEMORY", "Amount of memory in MiB")

	// Flags without environment variable support (pass empty string for envVarName)
	reg.StringWithEnv(&libvirtcfg.DataDir, "data-dir", defaultDataDir, "", "libvirt storage dir")
	reg.BoolWithEnv(&libvirtcfg.DisableCVM, "disable-cvm", false, "DISABLECVM", "Use non-CVMs for peer pods")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&libvirtcfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &libvirtcfg
}
