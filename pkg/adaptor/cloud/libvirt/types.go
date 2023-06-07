//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"net/netip"

	libvirt "libvirt.org/go/libvirt"
)

type Config struct {
	URI         string
	PoolName    string
	NetworkName string
	DataDir     string
	VolName     string
}

type vmConfig struct {
	name         string
	cpu          uint
	mem          uint
	rootDiskSize uint64
	userData     string
	metaData     string
	ips          []netip.Addr
	instanceId   string //keeping it consistent with sandbox.vsi
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
}
