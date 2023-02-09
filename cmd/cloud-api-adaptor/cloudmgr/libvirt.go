//go:build libvirt

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloudmgr

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/libvirt"
)

func init() {
	cloudTable["libvirt"] = &libvirtMgr{}
}

var libvirtcfg libvirt.Config

type libvirtMgr struct{}

func (_ *libvirtMgr) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&libvirtcfg.URI, "uri", "qemu:///system", "libvirt URI")
	flags.StringVar(&libvirtcfg.PoolName, "pool-name", "default", "libvirt storage pool")
	flags.StringVar(&libvirtcfg.NetworkName, "network-name", "default", "libvirt network pool")
	flags.StringVar(&libvirtcfg.DataDir, "data-dir", "/var/lib/libvirt/images", "libvirt storage dir")

}

func (_ *libvirtMgr) LoadEnv() {

}

func (_ *libvirtMgr) NewProvider() (cloud.Provider, error) {
	return libvirt.NewProvider(&libvirtcfg)
}
