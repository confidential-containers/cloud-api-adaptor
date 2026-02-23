//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"net/netip"

	libvirt "libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
)

type Config struct {
	URI            string
	PoolName       string
	NetworkName    string
	DataDir        string
	DisableCVM     bool
	VolName        string
	LaunchSecurity string
	Firmware       string
	CPU            uint
	Memory         uint // It stores the value in MiB
}

type vmConfig struct {
	name               string
	cpu                uint
	mem                uint // It stores the value in MiB
	rootDiskSize       uint64
	userData           string
	ips                []netip.Addr
	instanceID         string //keeping it consistent with sandbox.vsi
	launchSecurityType LaunchSecurityType
	firmware           string
}

type createDomainOutput struct {
	instance *vmConfig
}

type libvirtClient struct {
	connection *libvirt.Connect

	// storage pool that holds all volumes
	pool *libvirt.StoragePool
	// cache pool's name so we don't have to call failable GetName() method on pool all the time.
	poolName string

	// libvirt network name
	networkName string

	dataDir string

	volName string

	// information about the target node
	nodeInfo *libvirt.NodeInfo

	// host capabilities
	caps *libvirtxml.Caps
}

type LaunchSecurityType int

const (
	NoLaunchSecurity LaunchSecurityType = iota
	S390PV
)

func (l LaunchSecurityType) String() string {
	switch l {
	case NoLaunchSecurity:
		return "None"
	case S390PV:
		return "S390PV"
	default:
		return "unknown"
	}
}
