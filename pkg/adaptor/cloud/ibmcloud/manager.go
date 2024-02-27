// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
)

var ibmcloudVPCConfig Config

type Manager struct{}

func InitCloud() {
	cloud.AddCloud("ibmcloud", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&ibmcloudVPCConfig.ApiKey, "api-key", "", "IBM Cloud API key, defaults to `IBMCLOUD_API_KEY`")
	flags.StringVar(&ibmcloudVPCConfig.IAMProfileID, "iam-profile-id", "", "IBM IAM Profile ID, defaults to `IBMCLOUD_IAM_PROFILE_ID`")
	flags.StringVar(&ibmcloudVPCConfig.CRTokenFileName, "cr-token-filename", "/var/run/secrets/tokens/vault-token", "Projected service account token")
	flags.StringVar(&ibmcloudVPCConfig.IamServiceURL, "iam-service-url", "https://iam.cloud.ibm.com/identity/token", "IBM Cloud IAM Service URL")
	flags.StringVar(&ibmcloudVPCConfig.VpcServiceURL, "vpc-service-url", "https://jp-tok.iaas.cloud.ibm.com/v1", "IBM Cloud VPC Service URL")
	flags.StringVar(&ibmcloudVPCConfig.ResourceGroupID, "resource-group-id", "", "Resource Group ID")
	flags.StringVar(&ibmcloudVPCConfig.ProfileName, "profile-name", "", "Default instance profile name to be used for the Pod VMs")
	flags.Var(&ibmcloudVPCConfig.InstanceProfiles, "profile-list", "List of instance profile names to be used for the Pod VMs, comma separated")
	flags.StringVar(&ibmcloudVPCConfig.ZoneName, "zone-name", "", "Zone name")
	flags.Var(&ibmcloudVPCConfig.Images, "image-id", "List of Image IDs, comma separated")
	flags.StringVar(&ibmcloudVPCConfig.PrimarySubnetID, "primary-subnet-id", "", "Primary subnet ID")
	flags.StringVar(&ibmcloudVPCConfig.PrimarySecurityGroupID, "primary-security-group-id", "", "Primary security group ID")
	flags.StringVar(&ibmcloudVPCConfig.SecondarySubnetID, "secondary-subnet-id", "", "Secondary subnet ID")
	flags.StringVar(&ibmcloudVPCConfig.SecondarySecurityGroupID, "secondary-security-group-id", "", "Secondary security group ID")
	flags.StringVar(&ibmcloudVPCConfig.KeyID, "key-id", "", "SSH Key ID")
	flags.StringVar(&ibmcloudVPCConfig.VpcID, "vpc-id", "", "VPC ID")

}

func (_ *Manager) LoadEnv() {
	// overwrite config set by cmd parameters in oci image with env might come from orchastration platform
	cloud.DefaultToEnv(&ibmcloudVPCConfig.ApiKey, "IBMCLOUD_API_KEY", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.IAMProfileID, "IBMCLOUD_IAM_PROFILE_ID", "")

	cloud.DefaultToEnv(&ibmcloudVPCConfig.IamServiceURL, "IBMCLOUD_IAM_ENDPOINT", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.VpcServiceURL, "IBMCLOUD_VPC_ENDPOINT", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.ResourceGroupID, "IBMCLOUD_RESOURCE_GROUP_ID", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.ProfileName, "IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.ZoneName, "IBMCLOUD_ZONE", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.PrimarySubnetID, "IBMCLOUD_VPC_SUBNET_ID", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.PrimarySecurityGroupID, "IBMCLOUD_VPC_SG_ID", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.KeyID, "IBMCLOUD_SSH_KEY_ID", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.VpcID, "IBMCLOUD_VPC_ID", "")

	var instanceProfilesStr string
	cloud.DefaultToEnv(&instanceProfilesStr, "IBMCLOUD_PODVM_INSTANCE_PROFILE_LIST", "")
	if instanceProfilesStr != "" {
		_ = ibmcloudVPCConfig.InstanceProfiles.Set(instanceProfilesStr)
	}

	var imageIDsStr string
	cloud.DefaultToEnv(&imageIDsStr, "IBMCLOUD_PODVM_IMAGE_ID", "")
	if imageIDsStr != "" {
		_ = ibmcloudVPCConfig.Images.Set(imageIDsStr)
	}
}

func (_ *Manager) NewProvider() (cloud.Provider, error) {
	return NewProvider(&ibmcloudVPCConfig)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &ibmcloudVPCConfig
}
