package libvirt

import (
	"net"
)

type Config struct {
	URI         string
	PoolName    string
	NetworkName string
	DataDir     string
}

type vmConfig struct {
	name         string
	num          uint8
	cpu          uint
	mem          uint
	rootDiskSize uint64
	userData     string
	metaData     string
	ips          []net.IP
	instanceId   string //keeping it consistent with sandbox.vsi
}

type createInstanceOutput struct {
	instance *vmConfig
}
