// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var alibabacloudcfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("alibabacloud", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&alibabacloudcfg.AccessKeyId, "alibabacloud-access-key-id", "", "ALIBABACLOUD_ACCESS_KEY_ID", "Access Key ID")
	reg.StringWithEnv(&alibabacloudcfg.SecretKey, "alibabacloud-secret-access-key", "", "ALIBABACLOUD_ACCESS_KEY_SECRET", "Secret Key")
	reg.StringWithEnv(&alibabacloudcfg.Region, "region", "cn-beijing", "REGION", "Region")
	reg.StringWithEnv(&alibabacloudcfg.ImageId, "imageid", "", "IMAGEID", "Pod VM image id")
	reg.StringWithEnv(&alibabacloudcfg.InstanceType, "instance-type", "ecs.g8i.xlarge", "PODVM_INSTANCE_TYPE", "Pod VM instance type")
	reg.StringWithEnv(&alibabacloudcfg.VswitchId, "vswitch-id", "", "VSWITCH_ID", "vSwitch ID to be used for the Pod VMs")
	reg.StringWithEnv(&alibabacloudcfg.KeyName, "keyname", "", "KEYNAME", "SSH Keypair name to be used with the Pod VM")

	reg.BoolWithEnv(&alibabacloudcfg.UsePublicIP, "use-public-ip", false, "USE_PUBLIC_IP", "Use Public IP for connecting to the kata-agent inside the Pod VM")
	reg.IntWithEnv(&alibabacloudcfg.SystemDiskSize, "system-disk-size", 40, "SYSTEM_DISK_SIZE", "System Disk size (in GiB) for the Pod VMs")
	reg.BoolWithEnv(&alibabacloudcfg.DisableCVM, "disable-cvm", false, "DISABLECVM", "Use non-CVMs for peer pods")

	// Flags without environment variable support (pass empty string for envVarName)
	reg.StringWithEnv(&alibabacloudcfg.VpcId, "vpc-id", "", "", "VPC ID to be used for the Pod VMs")

	// Custom flag types (comma-separated lists)
	reg.CustomTypeWithEnv(&alibabacloudcfg.SecurityGroupIds, "security-group-ids", "cn-beijing", "SECURITY_GROUP_IDS", "Security Group Ids to be used for the Pod VM, comma separated")
	reg.CustomTypeWithEnv(&alibabacloudcfg.Tags, "tags", "", "TAGS", "Custom tags (key=value pairs) to be used for the Pod VMs, comma separated")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&alibabacloudcfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &alibabacloudcfg
}
