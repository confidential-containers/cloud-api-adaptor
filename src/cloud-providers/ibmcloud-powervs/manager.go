// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud_powervs

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var ibmcloudPowerVSConfig Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("ibmcloud-powervs", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&ibmcloudPowerVSConfig.ApiKey, "api-key", "", "IBMCLOUD_API_KEY", "IBM Cloud API key", provider.Secret(), provider.Required())
	reg.StringWithEnv(&ibmcloudPowerVSConfig.Zone, "zone", "", "POWERVS_ZONE", "PowerVS zone name", provider.Required())
	reg.StringWithEnv(&ibmcloudPowerVSConfig.ServiceInstanceID, "service-instance-id", "", "POWERVS_SERVICE_INSTANCE_ID", "ID of the PowerVS Service Instance", provider.Required())
	reg.StringWithEnv(&ibmcloudPowerVSConfig.NetworkID, "network-id", "", "POWERVS_NETWORK_ID", "ID of the network instance", provider.Required())
	reg.StringWithEnv(&ibmcloudPowerVSConfig.ImageId, "image-id", "", "POWERVS_IMAGE_ID", "ID of the boot image", provider.Required())
	reg.StringWithEnv(&ibmcloudPowerVSConfig.SSHKey, "ssh-key", "", "POWERVS_SSH_KEY_NAME", "Name of the SSH Key")
	reg.StringWithEnv(&ibmcloudPowerVSConfig.ProcessorType, "proc-type", "shared", "POWERVS_PROCESSOR_TYPE", "Name of the processor type")
	reg.StringWithEnv(&ibmcloudPowerVSConfig.SystemType, "sys-type", "s922", "POWERVS_SYSTEM_TYPE", "Name of the system type")
	reg.Float64WithEnv(&ibmcloudPowerVSConfig.Memory, "memory", 2, "POWERVS_MEMORY", "Amount of memory in GB")
	reg.Float64WithEnv(&ibmcloudPowerVSConfig.Processors, "cpu", 0.5, "POWERVS_PROCESSORS", "Number of processors allocated")
	reg.BoolWithEnv(&ibmcloudPowerVSConfig.UsePublicIP, "use-public-ip", false, "USE_PUBLIC_IP", "Use Public IP for connecting to the agent-protocol-forwarder inside the Pod VM")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&ibmcloudPowerVSConfig)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &ibmcloudPowerVSConfig
}
