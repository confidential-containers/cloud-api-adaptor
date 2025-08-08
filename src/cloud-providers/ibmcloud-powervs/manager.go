// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud_powervs

import (
	"flag"
	"strconv"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var ibmcloudPowerVSConfig Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("ibmcloud-powervs", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&ibmcloudPowerVSConfig.ApiKey, "api-key", "", "IBM Cloud API key, defaults to `IBMCLOUD_API_KEY`")
	flags.StringVar(&ibmcloudPowerVSConfig.Zone, "zone", "", "PowerVS zone name")
	flags.StringVar(&ibmcloudPowerVSConfig.ServiceInstanceID, "service-instance-id", "", "ID of the PowerVS Service Instance")
	flags.StringVar(&ibmcloudPowerVSConfig.NetworkID, "network-id", "", "ID of the network instance")
	flags.StringVar(&ibmcloudPowerVSConfig.ImageId, "image-id", "", "ID of the boot image")
	flags.StringVar(&ibmcloudPowerVSConfig.SSHKey, "ssh-key", "", "Name of the SSH Key")
	flags.Float64Var(&ibmcloudPowerVSConfig.Memory, "memory", 2, "Amount of memory in GB")
	flags.Float64Var(&ibmcloudPowerVSConfig.Processors, "cpu", 0.5, "Number of processors allocated")
	flags.StringVar(&ibmcloudPowerVSConfig.ProcessorType, "proc-type", "shared", "Name of the processor type")
	flags.StringVar(&ibmcloudPowerVSConfig.SystemType, "sys-type", "s922", "Name of the system type")
	flags.BoolVar(&ibmcloudPowerVSConfig.UsePublicIP, "use-public-ip", false, "Use Public IP for connecting to the agent-protocol-forwarder inside the Pod VM")
	flags.BoolVar(&ibmcloudPowerVSConfig.EnableSftp, "enable-sftp", false, "When set to true, transfers the user-data via SFTP")
	flags.StringVar(&ibmcloudPowerVSConfig.CloudUserName, "cloud-user-name", "", "name of the cloud user")

}

func (_ *Manager) LoadEnv() {
	// overwrite config set by cmd parameters in oci image with env might come from orchastration platform
	provider.DefaultToEnv(&ibmcloudPowerVSConfig.ApiKey, "IBMCLOUD_API_KEY", "")

	provider.DefaultToEnv(&ibmcloudPowerVSConfig.Zone, "POWERVS_ZONE", "")
	provider.DefaultToEnv(&ibmcloudPowerVSConfig.ServiceInstanceID, "POWERVS_SERVICE_INSTANCE_ID", "")
	provider.DefaultToEnv(&ibmcloudPowerVSConfig.NetworkID, "POWERVS_NETWORK_ID", "")
	provider.DefaultToEnv(&ibmcloudPowerVSConfig.ImageId, "POWERVS_IMAGE_ID", "")
	provider.DefaultToEnv(&ibmcloudPowerVSConfig.SSHKey, "POWERVS_SSH_KEY_NAME", "")
	provider.DefaultToEnv(&ibmcloudPowerVSConfig.ProcessorType, "POWERVS_PROCESSOR_TYPE", "")
	provider.DefaultToEnv(&ibmcloudPowerVSConfig.SystemType, "POWERVS_SYSTEM_TYPE", "")

	var memoryStr, processorsStr string
	provider.DefaultToEnv(&memoryStr, "POWERVS_MEMORY", "")
	if memoryStr != "" {
		ibmcloudPowerVSConfig.Memory, _ = strconv.ParseFloat(memoryStr, 64)
	}

	provider.DefaultToEnv(&processorsStr, "POWERVS_PROCESSORS", "")
	if processorsStr != "" {
		ibmcloudPowerVSConfig.Processors, _ = strconv.ParseFloat(processorsStr, 64)
	}
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&ibmcloudPowerVSConfig)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &ibmcloudPowerVSConfig
}
