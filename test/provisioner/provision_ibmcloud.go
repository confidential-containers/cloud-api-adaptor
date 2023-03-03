// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/test/utils"

	"github.com/IBM-Cloud/bluemix-go/api/container/containerv2"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	cosession "github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3/s3manager"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

func createVPC() error {
	if !IBMCloudProps.IsProvNewSubnet {
		return nil
	}

	classicAccess := false
	manual := "manual"

	options := &vpcv1.CreateVPCOptions{
		ResourceGroup: &vpcv1.ResourceGroupIdentity{
			ID: &IBMCloudProps.ResourceGroupID,
		},
		Name:                    &[]string{IBMCloudProps.VpcName}[0],
		ClassicAccess:           &classicAccess,
		AddressPrefixManagement: &manual,
	}
	vpcInstance, _, err := IBMCloudProps.VPC.CreateVPC(options)
	if err != nil {
		return err
	}

	IBMCloudProps.VpcID = *vpcInstance.ID

	if len(IBMCloudProps.VpcID) <= 0 {
		return errors.New("VpcID is empty, unknown error happened when create VPC.")
	}

	sgoptions := &vpcv1.GetVPCDefaultSecurityGroupOptions{}
	sgoptions.SetID(IBMCloudProps.VpcID)
	defaultSG, _, err := IBMCloudProps.VPC.GetVPCDefaultSecurityGroup(sgoptions)
	if err != nil {
		return err
	}

	IBMCloudProps.SecurityGroupID = *defaultSG.ID

	return nil
}

func deleteVPC() error {
	if !IBMCloudProps.IsProvNewVPC {
		return nil
	}

	deleteVpcOptions := &vpcv1.DeleteVPCOptions{}
	deleteVpcOptions.SetID(IBMCloudProps.VpcID)
	_, err := IBMCloudProps.VPC.DeleteVPC(deleteVpcOptions)

	if err != nil {
		return err
	}
	return nil
}

func createSubnet() error {
	if !IBMCloudProps.IsProvNewSubnet {
		return nil
	}

	cidrBlock := "10.0.1.0/24"
	options := &vpcv1.CreateSubnetOptions{}
	options.SetSubnetPrototype(&vpcv1.SubnetPrototype{
		Ipv4CIDRBlock: &cidrBlock,
		Name:          &[]string{IBMCloudProps.SubnetName}[0],
		VPC: &vpcv1.VPCIdentity{
			ID: &IBMCloudProps.VpcID,
		},
		Zone: &vpcv1.ZoneIdentity{
			Name: &IBMCloudProps.Zone,
		},
	})
	subnet, _, err := IBMCloudProps.VPC.CreateSubnet(options)
	if err != nil {
		return err
	}
	IBMCloudProps.SubnetID = *subnet.ID

	if len(IBMCloudProps.SubnetID) <= 0 {
		return errors.New("SubnetID is empty, unknown error happened when create Subnet.")
	}

	return nil
}

func deleteSubnet() error {
	if !IBMCloudProps.IsProvNewSubnet {
		return nil
	}

	options := &vpcv1.DeleteSubnetOptions{}
	options.SetID(IBMCloudProps.SubnetID)
	_, err := IBMCloudProps.VPC.DeleteSubnet(options)

	if err != nil {
		return err
	}
	return nil
}

func createVpcImpl() error {
	err := createVPC()
	if err != nil {
		return err
	}
	return createSubnet()
}

func deleteVpcImpl() error {
	err := deleteSubnet()
	if err != nil {
		return err
	}
	return deleteVPC()
}

// IBMCloudProvisioner implements the CloudProvisioner interface for ibmcloud.
type IBMCloudProvisioner struct {
}

func NewIBMCloudProvisioner(properties map[string]string) (CloudProvisioner, error) {
	if err := initProperties(properties); err != nil {
		return nil, err
	}

	if IBMCloudProps.IsSelfManaged {
		return &SelfManagedClusterProvisioner{}, nil
	}
	return &IBMCloudProvisioner{}, nil
}

// IBMCloudProvisioner

func (p *IBMCloudProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	clusterInfo := containerv2.ClusterCreateRequest{
		DisablePublicServiceEndpoint: true,
		Name:                         IBMCloudProps.ClusterName,
		Provider:                     "vpc-gen2",
		WorkerPools: containerv2.WorkerPoolConfig{
			CommonWorkerPoolConfig: containerv2.CommonWorkerPoolConfig{
				DiskEncryption: true,
				Flavor:         IBMCloudProps.WorkerFlavor,
				VpcID:          IBMCloudProps.VpcID,
				WorkerCount:    IBMCloudProps.WorkerCount,
				Zones: []containerv2.Zone{
					{
						ID:       IBMCloudProps.Zone,
						SubnetID: IBMCloudProps.SubnetID,
					},
				},
			},
		},
	}
	target := containerv2.ClusterTargetHeader{}
	_, err := IBMCloudProps.ClusterAPI.Create(clusterInfo, target)
	if err != nil {
		return err
	}

	return nil
}

func (p *IBMCloudProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return createVpcImpl()
}

func (p *IBMCloudProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	target := containerv2.ClusterTargetHeader{}
	err := IBMCloudProps.ClusterAPI.Delete(IBMCloudProps.ClusterName, target)
	if err != nil {
		return err
	}

	return nil
}

func (p *IBMCloudProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return deleteVpcImpl()
}

func (p *IBMCloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	filePath, err := filepath.Abs(imagePath)
	if err != nil {
		return err
	}

	conf := aws.NewConfig().
		WithEndpoint(IBMCloudProps.CosServiceURL).
		WithCredentials(ibmiam.NewStaticCredentials(aws.NewConfig(),
			IBMCloudProps.IamServiceURL, IBMCloudProps.ApiKey, IBMCloudProps.CosInstanceID)).
		WithS3ForcePathStyle(true)

	sess := cosession.Must(cosession.NewSession(conf))

	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	reader := &utils.CustomReader{
		Fp:      file,
		Size:    fileInfo.Size(),
		SignMap: map[int64]struct{}{},
	}

	uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
		u.PartSize = 5 * 1024 * 1024
		u.LeavePartsOnError = true
	})

	key := filepath.Base(filePath)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(IBMCloudProps.Bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		return err
	}

	var osNames []string
	if strings.EqualFold("s390x", IBMCloudProps.PodvmImageArch) {
		osNames = []string{"ubuntu-20-04-s390x"}
	} else {
		osNames = []string{"ubuntu-20-04-amd64"}

	}
	operatingSystemIdentityModel := &vpcv1.OperatingSystemIdentityByName{
		Name: &osNames[0],
	}

	cosID := "cos://" + IBMCloudProps.Region + "/" + IBMCloudProps.Bucket + "/" + key
	imageName := key
	options := &vpcv1.CreateImageOptions{}
	options.SetImagePrototype(&vpcv1.ImagePrototype{
		Name: &imageName,
		File: &vpcv1.ImageFilePrototype{
			Href: &cosID,
		},
		OperatingSystem: operatingSystemIdentityModel,
	})
	image, _, err := IBMCloudProps.VPC.CreateImage(options)
	if err != nil {
		return err
	}
	IBMCloudProps.PodvmImageID = *image.ID

	return nil
}

//func (p *IBMCloudProvisioner) DoKustomize(ctx context.Context, cfg *envconf.Config) error {
//	overlayFile := "../../install/overlays/ibmcloud/kustomization.yaml"
//	overlayFileBak := "../../install/overlays/ibmcloud/kustomization.yaml.bak"
//	err := os.Rename(overlayFile, overlayFileBak)
//	if err != nil {
//		return err
//	}
//
//	input, err := os.ReadFile(overlayFileBak)
//	if err != nil {
//		return err
//	}
//
//	replacer := strings.NewReplacer("IBMCLOUD_VPC_ENDPOINT=\"\"", "IBMCLOUD_VPC_ENDPOINT=\""+VpcServiceURL+"\"",
//		"IBMCLOUD_RESOURCE_GROUP_ID=\"\"", "IBMCLOUD_RESOURCE_GROUP_ID=\""+resourceGroupID+"\"",
//		"IBMCLOUD_SSH_KEY_ID=\"\"", "IBMCLOUD_SSH_KEY_ID=\""+SshKeyID+"\"",
//		"IBMCLOUD_PODVM_IMAGE_ID=\"\"", "IBMCLOUD_PODVM_IMAGE_ID=\""+PodvmImageID+"\"",
//		"IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME=\"\"", "IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME=\""+InstanceProfile+"\"",
//		"IBMCLOUD_ZONE=\"\"", "IBMCLOUD_ZONE=\""+Zone+"\"",
//		"IBMCLOUD_VPC_SUBNET_ID=\"\"", "IBMCLOUD_VPC_SUBNET_ID=\""+SubnetID+"\"",
//		"IBMCLOUD_VPC_SG_ID=\"\"", "IBMCLOUD_VPC_SG_ID=\""+SecurityGroupID+"\"",
//		"IBMCLOUD_VPC_ID=\"\"", "IBMCLOUD_VPC_ID=\""+VpcID+"\"",
//		"IBMCLOUD_API_KEY=\"\"", "IBMCLOUD_API_KEY=\""+ApiKey+"\"",
//		"IBMCLOUD_IAM_ENDPOINT=\"\"", "IBMCLOUD_IAM_ENDPOINT=\""+IamServiceURL+"\"")
//
//	output := replacer.Replace(string(input))
//
//	if err = os.WriteFile(overlayFile, []byte(output), 0666); err != nil {
//		return err
//	}
//
//	return nil
//}

func (p *IBMCloudProvisioner) GetVPCDefaultSecurityGroupID(vpcID string) (string, error) {
	if len(IBMCloudProps.SecurityGroupID) > 0 {
		return IBMCloudProps.SecurityGroupID, nil
	}

	options := &vpcv1.GetVPCDefaultSecurityGroupOptions{}
	options.SetID(vpcID)
	defaultSG, _, err := IBMCloudProps.VPC.GetVPCDefaultSecurityGroup(options)
	if err != nil {
		return "", err
	}

	return *defaultSG.ID, nil
}
