// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var awscfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("aws", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&awscfg.AccessKeyId, "aws-access-key-id", "", "AWS_ACCESS_KEY_ID", "Access Key ID")
	reg.StringWithEnv(&awscfg.SecretKey, "aws-secret-key", "", "AWS_SECRET_ACCESS_KEY", "Secret Key")
	reg.StringWithEnv(&awscfg.SessionToken, "aws-session-token", "", "AWS_SESSION_TOKEN", "Session Token")
	reg.StringWithEnv(&awscfg.InstanceType, "instance-type", "m6a.large", "PODVM_INSTANCE_TYPE", "Pod VM instance type")
	reg.StringWithEnv(&awscfg.Region, "aws-region", "", "AWS_REGION", "Region")
	reg.StringWithEnv(&awscfg.LaunchTemplateName, "aws-lt-name", "kata", "PODVM_LAUNCHTEMPLATE_NAME", "AWS Launch Template Name")
	reg.BoolWithEnv(&awscfg.UseLaunchTemplate, "use-lt", false, "USE_PODVM_LAUNCHTEMPLATE", "Use EC2 Launch Template for the Pod VMs")
	reg.StringWithEnv(&awscfg.ImageId, "imageid", "", "PODVM_AMI_ID", "Pod VM ami id")
	reg.StringWithEnv(&awscfg.KeyName, "keyname", "", "SSH_KP_NAME", "SSH Keypair name to be used with the Pod VM")
	reg.StringWithEnv(&awscfg.SubnetId, "subnetid", "", "AWS_SUBNET_ID", "Subnet ID to be used for the Pod VMs")
	reg.BoolWithEnv(&awscfg.UsePublicIP, "use-public-ip", false, "USE_PUBLIC_IP", "Use Public IP for connecting to the kata-agent inside the Pod VM")
	reg.IntWithEnv(&awscfg.RootVolumeSize, "root-volume-size", 30, "ROOT_VOLUME_SIZE", "Root volume size (in GiB) for the Pod VMs")
	reg.BoolWithEnv(&awscfg.DisableCVM, "disable-cvm", false, "DISABLECVM", "Use non-CVMs for peer pods")

	// Flags without environment variable support (pass empty string for envVarName)
	reg.StringWithEnv(&awscfg.LoginProfile, "aws-profile", "", "", "AWS Login Profile")

	// Custom flag types (comma-separated lists)
	reg.CustomTypeWithEnv(&awscfg.SecurityGroupIds, "securitygroupids", "", "AWS_SG_IDS", "Security Group Ids to be used for the Pod VM, comma separated")
	reg.CustomTypeWithEnv(&awscfg.InstanceTypes, "instance-types", "", "PODVM_INSTANCE_TYPES", "Instance types to be used for the Pod VMs, comma separated")
	reg.CustomTypeWithEnv(&awscfg.Tags, "tags", "", "TAGS", "Custom tags (key=value pairs) to be used for the Pod VMs, comma separated")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&awscfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &awscfg
}
