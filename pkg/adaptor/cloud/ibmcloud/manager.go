// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
)

var ibmcloudConfig Config

type Manager struct{}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&ibmcloudConfig.ApiKey, "api-key", "", "IBM Cloud API key, defaults to `IBMCLOUD_API_KEY`")
	flags.StringVar(&ibmcloudConfig.IamServiceURL, "iam-service-url", "https://iam.cloud.ibm.com/identity/token", "IBM Cloud IAM Service URL")
	flags.StringVar(&ibmcloudConfig.VpcServiceURL, "vpc-service-url", "https://jp-tok.iaas.cloud.ibm.com/v1", "IBM Cloud VPC Service URL")
	flags.StringVar(&ibmcloudConfig.ResourceGroupID, "resource-group-id", "", "Resource Group ID")
	flags.StringVar(&ibmcloudConfig.ProfileName, "profile-name", "", "Profile name")
	flags.StringVar(&ibmcloudConfig.ZoneName, "zone-name", "", "Zone name")
	flags.StringVar(&ibmcloudConfig.ImageID, "image-id", "", "Image ID")
	flags.StringVar(&ibmcloudConfig.PrimarySubnetID, "primary-subnet-id", "", "Primary subnet ID")
	flags.StringVar(&ibmcloudConfig.PrimarySecurityGroupID, "primary-security-group-id", "", "Primary security group ID")
	flags.StringVar(&ibmcloudConfig.SecondarySubnetID, "secondary-subnet-id", "", "Secondary subnet ID")
	flags.StringVar(&ibmcloudConfig.SecondarySecurityGroupID, "secondary-security-group-id", "", "Secondary security group ID")
	flags.StringVar(&ibmcloudConfig.KeyID, "key-id", "", "SSH Key ID")
	flags.StringVar(&ibmcloudConfig.VpcID, "vpc-id", "", "VPC ID")

}

func (_ *Manager) LoadEnv() {
	cloud.DefaultToEnv(&ibmcloudConfig.ApiKey, "IBMCLOUD_API_KEY", "")
}

func (_ *Manager) NewProvider() (cloud.Provider, error) {
	return NewProvider(&ibmcloudConfig)
}
