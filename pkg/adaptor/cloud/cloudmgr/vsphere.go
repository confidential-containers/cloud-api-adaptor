//go:build vsphere

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloudmgr

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/vsphere"
)

func init() {
	cloudTable["vsphere"] = &vsphereMgr{}
}

var vspherecfg vsphere.Config

type vsphereMgr struct{}

func (_ *vsphereMgr) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&vspherecfg.VcenterURL, "vcenter-url", "", "URL of vCenter instance to connect to")
	flags.StringVar(&vspherecfg.UserName, "user-name", "", "vCenter Username")
	flags.StringVar(&vspherecfg.Password, "password", "", "vCenter Password")
	flags.StringVar(&vspherecfg.Thumbprint, "thumbprint", "", "SHA1 thumbprint of the vcenter certificate. Enable verification of certificate chain and host name.")
	flags.StringVar(&vspherecfg.Template, "template", "podvm-template", "vCenter template to deploy")
	flags.StringVar(&vspherecfg.Datacenter, "data-center", "", "vCenter destination datacenter name")
	flags.StringVar(&vspherecfg.Datastore, "data-store", "", "vCenter datastore")
	flags.StringVar(&vspherecfg.Deployfolder, "deploy-folder", "", "vCenter vm destination folder relative to the vm inventory path (your-data-center/vm). \nExample '-deploy-folder peerods' will create or use the existing folder peerpods as the \ndeploy-folder in /datacenter/vm/peerpods")
	flags.StringVar(&vspherecfg.Cluster, "cluster", "", "vCenter destination cluster name ")
	flags.StringVar(&vspherecfg.DRS, "drs", "false", "Use DRS for clone placement in destination Vcenter cluster")
	flags.StringVar(&vspherecfg.Host, "host", "", "vCenter host name of resource pool destination")
}

func (_ *vsphereMgr) LoadEnv() {
	defaultToEnv(&vspherecfg.UserName, "GOVC_USERNAME")
	defaultToEnv(&vspherecfg.Password, "GOVC_PASSWORD")
	defaultToEnv(&vspherecfg.Thumbprint, "GOVC_THUMBPRINT")
}

func (_ *vsphereMgr) NewProvider() (cloud.Provider, error) {
	return vsphere.NewProvider(&vspherecfg)
}
