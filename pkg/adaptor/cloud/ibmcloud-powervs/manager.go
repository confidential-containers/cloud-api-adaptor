// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"strconv"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
)

var ibmcloudPowerVSConfig Config

type Manager struct{}

func InitCloud() {
	cloud.AddCloud("ibmcloud-powervs", &Manager{})
}

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
	flags.BoolVar(&ibmcloudPowerVSConfig.UsePublicIP, "use-public-ip", false, "Use Public IP for connecting to the agent-protocol-forwarder inside the Pod VM")

}

func (_ *Manager) LoadEnv() {
	// overwrite config set by cmd parameters in oci image with env might come from orchastration platform
	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.ApiKey, "IBMCLOUD_API_KEY", "")

	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.Zone, "POWERVS_ZONE", "")
	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.ServiceInstanceID, "POWERVS_SERVICE_INSTANCE_ID", "")
	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.NetworkID, "POWERVS_NETWORK_ID", "")
	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.ImageID, "POWERVS_IMAGE_ID", "")
	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.SSHKey, "POWERVS_SSH_KEY_NAME", "")
	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.ProcessorType, "POWERVS_PROCESSOR_TYPE", "")
	cloud.DefaultToEnv(&ibmcloudPowerVSConfig.SystemType, "POWERVS_SYSTEM_TYPE", "")

	var memoryStr, processorsStr string
	cloud.DefaultToEnv(&memoryStr, "POWERVS_MEMORY", "")
	if memoryStr != "" {
		ibmcloudPowerVSConfig.Memory, _ = strconv.ParseFloat(memoryStr, 64)
	}

	cloud.DefaultToEnv(&processorsStr, "POWERVS_MEMORY", "")
	if processorsStr != "" {
		ibmcloudPowerVSConfig.Processors, _ = strconv.ParseFloat(processorsStr, 64)
	}
}

func (_ *Manager) NewProvider() (cloud.Provider, error) {
	return NewProvider(&ibmcloudPowerVSConfig)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &ibmcloudPowerVSConfig
}
