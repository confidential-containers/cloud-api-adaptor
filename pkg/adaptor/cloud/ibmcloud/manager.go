// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
)

var ibmcloudVPCConfig Config

type Manager struct{}

func (*Manager) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&ibmcloudVPCConfig.ApiKey, "api-key", "", "IBM Cloud API key, defaults to `IBMCLOUD_API_KEY`")
	flags.StringVar(&ibmcloudVPCConfig.IAMProfileID, "iam-profile-id", "", "IBM IAM Profile ID, defaults to `IBMCLOUD_IAM_PROFILE_ID`")
	flags.StringVar(&ibmcloudVPCConfig.CRTokenFileName, "cr-token-filename", "/var/run/secrets/tokens/vault-token", "Projected service account token")
	flags.StringVar(&ibmcloudVPCConfig.IamServiceURL, "iam-service-url", "https://iam.cloud.ibm.com/identity/token", "IBM Cloud IAM Service URL")
	flags.StringVar(&ibmcloudVPCConfig.VpcServiceURL, "vpc-service-url", "https://jp-tok.iaas.cloud.ibm.com/v1", "IBM Cloud VPC Service URL")
	flags.StringVar(&ibmcloudVPCConfig.ResourceGroupID, "resource-group-id", "", "Resource Group ID")
	flags.StringVar(&ibmcloudVPCConfig.ProfileName, "profile-name", "", "Profile name")
	flags.StringVar(&ibmcloudVPCConfig.ZoneName, "zone-name", "", "Zone name")
	flags.StringVar(&ibmcloudVPCConfig.ImageID, "image-id", "", "Image ID")
	flags.StringVar(&ibmcloudVPCConfig.PrimarySubnetID, "primary-subnet-id", "", "Primary subnet ID")
	flags.StringVar(&ibmcloudVPCConfig.PrimarySecurityGroupID, "primary-security-group-id", "", "Primary security group ID")
	flags.StringVar(&ibmcloudVPCConfig.SecondarySubnetID, "secondary-subnet-id", "", "Secondary subnet ID")
	flags.StringVar(&ibmcloudVPCConfig.SecondarySecurityGroupID, "secondary-security-group-id", "", "Secondary security group ID")
	flags.StringVar(&ibmcloudVPCConfig.KeyID, "key-id", "", "SSH Key ID")
	flags.StringVar(&ibmcloudVPCConfig.VpcID, "vpc-id", "", "VPC ID")

}

func (*Manager) LoadEnv() {
	cloud.DefaultToEnv(&ibmcloudVPCConfig.ApiKey, "IBMCLOUD_API_KEY", "")
	cloud.DefaultToEnv(&ibmcloudVPCConfig.IAMProfileID, "IBMCLOUD_IAM_PROFILE_ID", "")
}

func (*Manager) NewProvider() (cloud.Provider, error) {
	return NewProvider(&ibmcloudVPCConfig)
}
