// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var vspherecfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("vsphere", &Manager{})
}

func (*Manager) ParseCmd(flags *flag.FlagSet) {

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

func (*Manager) LoadEnv() {
	provider.DefaultToEnv(&vspherecfg.UserName, "GOVC_USERNAME", "")
	provider.DefaultToEnv(&vspherecfg.Password, "GOVC_PASSWORD", "")
	provider.DefaultToEnv(&vspherecfg.Thumbprint, "GOVC_THUMBPRINT", "")
}

func (*Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&vspherecfg)
}

func (*Manager) GetConfig() (config *Config) {
	return &vspherecfg
}
