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

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&vspherecfg.UserName, "user-name", "", "GOVC_USERNAME", "vCenter Username")
	reg.StringWithEnv(&vspherecfg.Password, "password", "", "GOVC_PASSWORD", "vCenter Password")
	reg.StringWithEnv(&vspherecfg.Thumbprint, "thumbprint", "", "GOVC_THUMBPRINT", "SHA1 thumbprint of the vcenter certificate. Enable verification of certificate chain and host name.")
	reg.StringWithEnv(&vspherecfg.VcenterURL, "vcenter-url", "", "GOVC_URL", "URL of vCenter instance to connect to")
	reg.StringWithEnv(&vspherecfg.Template, "template", "podvm-template", "GOVC_TEMPLATE", "vCenter template to deploy")
	reg.StringWithEnv(&vspherecfg.Datacenter, "data-center", "", "GOVC_DATACENTER", "vCenter destination datacenter name")
	reg.StringWithEnv(&vspherecfg.Datastore, "data-store", "", "GOVC_DATASTORE", "vCenter datastore")
	reg.StringWithEnv(&vspherecfg.Deployfolder, "deploy-folder", "", "GOVC_FOLDER", "vCenter vm destination folder relative to the vm inventory path (your-data-center/vm). \nExample '-deploy-folder peerods' will create or use the existing folder peerpods as the \ndeploy-folder in /datacenter/vm/peerpods")
	reg.StringWithEnv(&vspherecfg.Cluster, "cluster", "", "GOVC_VCLUSTER", "vCenter destination cluster name ")
	reg.StringWithEnv(&vspherecfg.DRS, "drs", "false", "GOVC_DRS", "Use DRS for clone placement in destination Vcenter cluster")
	reg.StringWithEnv(&vspherecfg.Host, "host", "", "GOVC_HOST", "vCenter host name of resource pool destination")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&vspherecfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &vspherecfg
}
