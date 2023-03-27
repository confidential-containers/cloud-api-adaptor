// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud_powervs

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
)

var ibmcloudPowerVSConfig Config

type Manager struct{}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&ibmcloudPowerVSConfig.ApiKey, "api-key", "", "IBM Cloud API key, defaults to `IBMCLOUD_API_KEY`")
	flags.StringVar(&ibmcloudPowerVSConfig.Zone, "zone", "", "PowerVS zone name")
	flags.StringVar(&ibmcloudPowerVSConfig.ServiceInstanceID, "service-instance-id", "", "ID of the PowerVS Service Instance")
	flags.StringVar(&ibmcloudPowerVSConfig.NetworkID, "network-id", "", "ID of the network instance")
	flags.StringVar(&ibmcloudPowerVSConfig.ImageID, "image-id", "", "ID of the boot image")
	flags.StringVar(&ibmcloudPowerVSConfig.SSHKey, "ssh-key", "", "Name of the SSH Key")
	flags.Float64Var(&ibmcloudPowerVSConfig.Memory, "memory", 2, "Amount of memory in GB")
	flags.Float64Var(&ibmcloudPowerVSConfig.Processors, "cpu", 0.5, "Number of processors allocated")
	flags.StringVar(&ibmcloudPowerVSConfig.ProcessorType, "proc-type", "shared", "Name of the processor type")
	flags.StringVar(&ibmcloudPowerVSConfig.SystemType, "sys-type", "s922", "Name of the system type")

}

func (_ *Manager) LoadEnv() {
	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.ApiKey, "IBMCLOUD_API_KEY", "")
}

func (_ *Manager) NewProvider() (cloud.Provider, error) {
	return NewProvider(&ibmcloudPowerVSConfig)
}
