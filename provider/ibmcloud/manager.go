// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"flag"
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/confidential-containers/cloud-api-adaptor/provider"
)

var ibmcloudVPCConfig Config

type Manager struct {
	service *vpcv1.VpcV1
}

func init() {
	provider.AddCloudProvider("ibmcloud", &Manager{})
}

func (*Manager) ParseCmd(flags *flag.FlagSet) {

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

func (m *Manager) LoadEnv(extras map[string]string) error {

	provider.DefaultToEnv(&ibmcloudVPCConfig.ApiKey, "IBMCLOUD_API_KEY", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.IAMProfileID, "IBMCLOUD_IAM_PROFILE_ID", "")

	var authenticator core.Authenticator

	if ibmcloudVPCConfig.ApiKey != "" {
		authenticator = &core.IamAuthenticator{
			ApiKey: ibmcloudVPCConfig.ApiKey,
			URL:    ibmcloudVPCConfig.IamServiceURL,
		}
	} else if ibmcloudVPCConfig.IAMProfileID != "" {
		authenticator = &core.ContainerAuthenticator{
			URL:             ibmcloudVPCConfig.IamServiceURL,
			IAMProfileID:    ibmcloudVPCConfig.IAMProfileID,
			CRTokenFilename: ibmcloudVPCConfig.CRTokenFileName,
		}
	} else {
		return fmt.Errorf("either an IAM API Key or Profile ID needs to be set")
	}

	nodeRegion, ok := extras["topology.kubernetes.io/region"]
	if ibmcloudVPCConfig.VpcServiceURL == "" && ok {
		// Assume in prod if fetching from labels for now
		// TODO handle other environments
		ibmcloudVPCConfig.VpcServiceURL = fmt.Sprintf("https://%s.iaas.cloud.ibm.com/v1", nodeRegion)
	}

	var err error
	m.service, err = vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: authenticator,
		URL:           ibmcloudVPCConfig.VpcServiceURL,
	})

	if err != nil {
		return err
	}

	// If this label exists assume we are in an IKS cluster
	primarySubnetID, iks := extras["ibm-cloud.kubernetes.io/subnet-id"]
	if iks {
		if ibmcloudVPCConfig.ZoneName == "" {
			ibmcloudVPCConfig.ZoneName = extras["topology.kubernetes.io/zone"]
		}
		vpcID, rgID, sgID, err := fetchVPCDetails(m.service, primarySubnetID)
		if err != nil {
			logger.Printf("warning, unable to automatically populate VPC details\ndue to: %v\n", err)
		} else {
			if ibmcloudVPCConfig.PrimarySubnetID == "" {
				ibmcloudVPCConfig.PrimarySubnetID = primarySubnetID
			}
			if ibmcloudVPCConfig.VpcID == "" {
				ibmcloudVPCConfig.VpcID = vpcID
			}
			if ibmcloudVPCConfig.ResourceGroupID == "" {
				ibmcloudVPCConfig.ResourceGroupID = rgID
			}
			if ibmcloudVPCConfig.PrimarySecurityGroupID == "" {
				ibmcloudVPCConfig.PrimarySecurityGroupID = sgID
			}
		}
	}

	// overwrite ibmcloudVPCConfig set by cmd parameters in oci image with env might come from orchastration platform

	provider.DefaultToEnv(&ibmcloudVPCConfig.IamServiceURL, "IBMCLOUD_IAM_ENDPOINT", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.VpcServiceURL, "IBMCLOUD_VPC_ENDPOINT", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.ResourceGroupID, "IBMCLOUD_RESOURCE_GROUP_ID", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.ProfileName, "IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.ZoneName, "IBMCLOUD_ZONE", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.PrimarySubnetID, "IBMCLOUD_VPC_SUBNET_ID", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.PrimarySecurityGroupID, "IBMCLOUD_VPC_SG_ID", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.KeyID, "IBMCLOUD_SSH_KEY_ID", "")
	provider.DefaultToEnv(&ibmcloudVPCConfig.VpcID, "IBMCLOUD_VPC_ID", "")

	var instanceProfilesStr string
	provider.DefaultToEnv(&instanceProfilesStr, "IBMCLOUD_PODVM_INSTANCE_PROFILE_LIST", "")
	if instanceProfilesStr != "" {
		_ = ibmcloudVPCConfig.InstanceProfiles.Set(instanceProfilesStr)
	}

	var imageIDsStr string
	provider.DefaultToEnv(&imageIDsStr, "IBMCLOUD_PODVM_IMAGE_ID", "")
	if imageIDsStr != "" {
		_ = ibmcloudVPCConfig.Images.Set(imageIDsStr)
	}
	return nil
}

func fetchVPCDetails(vpcV1 *vpcv1.VpcV1, subnetID string) (vpcID string, resourceGroupID string, securityGroupID string, e error) {
	subnet, response, err := vpcV1.GetSubnet(&vpcv1.GetSubnetOptions{
		ID: &subnetID,
	})
	if err != nil {
		e = fmt.Errorf("VPC error with:\n %w\nfurther details:\n %v", err, response)
		return
	}

	sg, response, err := vpcV1.GetVPCDefaultSecurityGroup(&vpcv1.GetVPCDefaultSecurityGroupOptions{
		ID: subnet.VPC.ID,
	})
	if err != nil {
		e = fmt.Errorf("VPC error with:\n %w\nfurther details:\n %v", err, response)
		return
	}

	securityGroupID = *sg.ID
	vpcID = *subnet.VPC.ID
	resourceGroupID = *subnet.ResourceGroup.ID
	return
}

func (m *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&ibmcloudVPCConfig, m.service)
}
