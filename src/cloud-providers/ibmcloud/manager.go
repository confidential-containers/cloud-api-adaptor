// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var ibmcloudVPCConfig Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("ibmcloud", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&ibmcloudVPCConfig.ApiKey, "api-key", "", "IBMCLOUD_API_KEY", "IBM Cloud API key", provider.Secret())
	reg.StringWithEnv(&ibmcloudVPCConfig.IAMProfileID, "iam-profile-id", "", "IBMCLOUD_IAM_PROFILE_ID", "IBM IAM Profile ID", provider.Secret())
	reg.StringWithEnv(&ibmcloudVPCConfig.IamServiceURL, "iam-service-url", "https://iam.cloud.ibm.com/identity/token", "IBMCLOUD_IAM_ENDPOINT", "IBM Cloud IAM Service URL")
	reg.StringWithEnv(&ibmcloudVPCConfig.VpcServiceURL, "vpc-service-url", "", "IBMCLOUD_VPC_ENDPOINT", "IBM Cloud VPC Service URL")
	reg.StringWithEnv(&ibmcloudVPCConfig.ResourceGroupID, "resource-group-id", "", "IBMCLOUD_RESOURCE_GROUP_ID", "Resource Group ID")
	reg.StringWithEnv(&ibmcloudVPCConfig.ProfileName, "profile-name", "", "IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME", "Default instance profile name to be used for the Pod VMs")
	reg.StringWithEnv(&ibmcloudVPCConfig.ZoneName, "zone-name", "", "IBMCLOUD_ZONE", "Zone name")
	reg.StringWithEnv(&ibmcloudVPCConfig.PrimarySubnetID, "primary-subnet-id", "", "IBMCLOUD_VPC_SUBNET_ID", "Primary subnet ID")
	reg.StringWithEnv(&ibmcloudVPCConfig.KeyID, "key-id", "", "IBMCLOUD_SSH_KEY_ID", "SSH Key ID")
	reg.StringWithEnv(&ibmcloudVPCConfig.VpcID, "vpc-id", "", "IBMCLOUD_VPC_ID", "VPC ID")
	reg.StringWithEnv(&ibmcloudVPCConfig.ClusterID, "cluster-id", "", "IBMCLOUD_CLUSTER_ID", "Cluster ID")

	reg.BoolWithEnv(&ibmcloudVPCConfig.DisableCVM, "disable-cvm", true, "DISABLECVM", "Use non-CVMs for peer pods")

	// Flags without environment variable support (pass empty string for envVarName)
	reg.StringWithEnv(&ibmcloudVPCConfig.CRTokenFileName, "cr-token-filename", "/var/run/secrets/tokens/vault-token", "", "Projected service account token")
	reg.StringWithEnv(&ibmcloudVPCConfig.SecondarySubnetID, "secondary-subnet-id", "", "", "Secondary subnet ID")
	reg.StringWithEnv(&ibmcloudVPCConfig.SecondarySecurityGroupID, "secondary-security-group-id", "", "", "Secondary security group ID")

	// Custom flag types (comma-separated lists)
	reg.CustomTypeWithEnv(&ibmcloudVPCConfig.InstanceProfiles, "profile-list", "", "IBMCLOUD_PODVM_INSTANCE_PROFILE_LIST", "List of instance profile names to be used for the Pod VMs, comma separated")
	reg.CustomTypeWithEnv(&ibmcloudVPCConfig.Images, "image-id", "", "IBMCLOUD_PODVM_IMAGE_ID", "List of Image IDs, comma separated", provider.Required())
	reg.CustomTypeWithEnv(&ibmcloudVPCConfig.Tags, "tags", "", "TAGS", "List of tags to attach to the Pod VMs, comma separated")
	reg.CustomTypeWithEnv(&ibmcloudVPCConfig.SecurityGroupIds, "security-group-ids", "", "IBMCLOUD_SECURITY_GROUP_IDS", "List of additional Security Group IDs to be used for the Pod VM, comma separated (cluster security group is automatically added)")
	reg.CustomTypeWithEnv(&ibmcloudVPCConfig.DedicatedHostIDs, "dedicated-host-ids", "", "IBMCLOUD_DEDICATED_HOST_IDS", "List of Dedicated Host IDs, provide one from each Zone")
	reg.CustomTypeWithEnv(&ibmcloudVPCConfig.DedicatedHostGroupIDs, "dedicated-host-group-ids", "", "IBMCLOUD_DEDICATED_HOST_GROUP_IDS", "List of Dedicated Host Group IDs, provide one from each Zone")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&ibmcloudVPCConfig)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &ibmcloudVPCConfig
}
