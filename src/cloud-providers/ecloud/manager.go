// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ecloud

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var ecloudcfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("ecloud", &Manager{})
}

func (*Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&ecloudcfg.AccessKeyID, "ecloud-access-key-id", "", "ECLOUD_ACCESS_KEY_ID", "Access Key ID", provider.Secret())
	reg.StringWithEnv(&ecloudcfg.SecretKey, "ecloud-secret-access-key", "", "ECLOUD_ACCESS_KEY_SECRET", "Secret Key", provider.Secret())
	reg.StringWithEnv(&ecloudcfg.Region, "region", "cn-jiangsu-1", "REGION", "Region ID, e.g. cn-jiangsu-1")
	reg.StringWithEnv(&ecloudcfg.ZoneID, "zone-id", "", "ECLOUD_ZONE_ID", "Availability zone ID, e.g. cn-jiangsu-1a")
	reg.StringWithEnv(&ecloudcfg.ImageID, "imageid", "", "IMAGEID", "Pod VM image id", provider.Required())
	reg.StringWithEnv(&ecloudcfg.InstanceType, "instance-type", "s1.large.2", "PODVM_INSTANCE_TYPE", "Pod VM instance type (flavor name)")
	reg.StringWithEnv(&ecloudcfg.NetworkID, "network-id", "", "ECLOUD_NETWORK_ID", "Private network (subnet) ID to be used for the Pod VMs", provider.Required())
	reg.StringWithEnv(&ecloudcfg.KeyName, "keyname", "", "KEYNAME", "SSH Keypair name to be used with the Pod VM")

	reg.BoolWithEnv(&ecloudcfg.UsePublicIP, "use-public-ip", false, "USE_PUBLIC_IP", "Use Public IP for connecting to the kata-agent inside the Pod VM")
	reg.IntWithEnv(&ecloudcfg.PublicBandwidthSize, "public-bandwidth-size", 100, "ECLOUD_PUBLIC_BANDWIDTH_SIZE", "Public IP bandwidth size (in Mbps) for the Pod VMs")
	reg.IntWithEnv(&ecloudcfg.SystemDiskSize, "system-disk-size", 40, "SYSTEM_DISK_SIZE", "System Disk size (in GiB) for the Pod VMs")
	reg.StringWithEnv(&ecloudcfg.SystemDiskType, "system-disk-type", "highPerformance", "ECLOUD_SYSTEM_DISK_TYPE", "System Disk type (e.g. highPerformance, ssd) for the Pod VMs")

	// Custom flag types (comma-separated lists)
	reg.CustomTypeWithEnv(&ecloudcfg.SecurityGroupIDs, "security-group-ids", "", "SECURITY_GROUP_IDS", "Security Group Ids to be used for the Pod VM, comma separated")
	reg.CustomTypeWithEnv(&ecloudcfg.InstanceTypes, "instance-types", "", "PODVM_INSTANCE_TYPES", "Instance types (flavor names) to be used for the Pod VMs, comma separated")
	reg.CustomTypeWithEnv(&ecloudcfg.Tags, "tags", "", "TAGS", "Custom tags (key=value pairs) to be used for the Pod VMs, comma separated")
}

func (*Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (*Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&ecloudcfg)
}

func (*Manager) GetConfig() (config *Config) {
	return &ecloudcfg
}
