// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"strings"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/test/utils"

	"github.com/IBM-Cloud/bluemix-go/api/container/containerv2"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	cosession "github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3/s3manager"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	log "github.com/sirupsen/logrus"
)

// https://cloud.ibm.com/docs/vpc?topic=vpc-configuring-address-prefixes
func getCidrBlock(region, zone string) string {
	switch region {
	case "us-south":
		switch zone {
		case region + "-1":
			return "10.240.0.0/18"
		case region + "-2":
			return "10.240.64.0/18"
		case region + "-3":
			return "10.240.128.0/18"
		}
	case "us-east":
		switch zone {
		case region + "-1":
			return "10.241.0.0/18"
		case region + "-2":
			return "10.241.64.0/18"
		case region + "-3":
			return "10.241.128.0/18"
		}
	case "eu-gb":
		switch zone {
		case region + "-1":
			return "10.242.0.0/18"
		case region + "-2":
			return "10.242.64.0/18"
		case region + "-3":
			return "10.242.128.0/18"
		}
	case "eu-de":
		switch zone {
		case region + "-1":
			return "10.243.0.0/18"
		case region + "-2":
			return "10.243.64.0/18"
		case region + "-3":
			return "10.243.128.0/18"
		}
	case "jp-tok":
		switch zone {
		case region + "-1":
			return "10.244.0.0/18"
		case region + "-2":
			return "10.244.64.0/18"
		case region + "-3":
			return "10.244.128.0/18"
		}
	case "au-syd":
		switch zone {
		case region + "-1":
			return "10.245.0.0/18"
		case region + "-2":
			return "10.245.64.0/18"
		case region + "-3":
			return "10.245.128.0/18"
		}
	case "jp-osa":
		switch zone {
		case region + "-1":
			return "10.248.0.0/18"
		case region + "-2":
			return "10.248.64.0/18"
		case region + "-3":
			return "10.248.128.0/18"
		}
	case "ca-tor":
		switch zone {
		case region + "-1":
			return "10.249.0.0/18"
		case region + "-2":
			return "10.249.64.0/18"
		case region + "-3":
			return "10.249.128.0/18"
		}
	case "br-sao":
		switch zone {
		case region + "-1":
			return "10.250.0.0/18"
		case region + "-2":
			return "10.250.64.0/18"
		case region + "-3":
			return "10.250.128.0/18"
		}
	}
	return ""
}

func createVPC() error {
	if !IBMCloudProps.IsProvNewSubnet {
		fmt.Printf("Using existing VPC: %s\n", IBMCloudProps.VpcID)
		return nil
	}

	classicAccess := false
	//manual := "manual"

	options := &vpcv1.CreateVPCOptions{
		ResourceGroup: &vpcv1.ResourceGroupIdentity{
			ID: &IBMCloudProps.ResourceGroupID,
		},
		Name:          &[]string{IBMCloudProps.VpcName}[0],
		ClassicAccess: &classicAccess,
		//AddressPrefixManagement: &manual,
	}
	log.Infof("Creating VPC %s in ResourceGroupID %s.\n", IBMCloudProps.VpcName, IBMCloudProps.ResourceGroupID)
	vpcInstance, _, err := IBMCloudProps.VPC.CreateVPC(options)
	if err != nil {
		return err
	}

	IBMCloudProps.VpcID = *vpcInstance.ID
	log.Infof("Created VPC with ID %s in ResourceGroupID %s.\n", IBMCloudProps.VpcID, IBMCloudProps.ResourceGroupID)

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
	log.Infof("Got VPC default SecurityGroupID %s.\n", IBMCloudProps.SecurityGroupID)

	return nil
}

func deleteVPC() error {
	if !IBMCloudProps.IsProvNewVPC {
		fmt.Printf("Do not delete because using existing VPC: %s\n", IBMCloudProps.VpcID)
		return nil
	}

	deleteVpcOptions := &vpcv1.DeleteVPCOptions{}
	deleteVpcOptions.SetID(IBMCloudProps.VpcID)
	log.Infof("Deleting VPC with ID %s.\n", IBMCloudProps.VpcID)
	_, err := IBMCloudProps.VPC.DeleteVPC(deleteVpcOptions)

	if err != nil {
		return err
	}
	log.Infof("Deleted VPC with ID %s.\n", IBMCloudProps.VpcID)
	return nil
}

func createSubnet() error {
	if !IBMCloudProps.IsProvNewSubnet {
		fmt.Printf("Using existing Subnet: %s\n", IBMCloudProps.SubnetID)
		return nil
	}

	cidrBlock := getCidrBlock(IBMCloudProps.Region, IBMCloudProps.Zone)
	if cidrBlock == "" {
		return errors.New("Can not calculate cidrBlock from Region and Zone.")
	}
	options := &vpcv1.CreateSubnetOptions{}
	options.SetSubnetPrototype(&vpcv1.SubnetPrototype{
		ResourceGroup: &vpcv1.ResourceGroupIdentity{
			ID: &IBMCloudProps.ResourceGroupID,
		},
		Ipv4CIDRBlock: &cidrBlock,
		Name:          &[]string{IBMCloudProps.SubnetName}[0],
		VPC: &vpcv1.VPCIdentity{
			ID: &IBMCloudProps.VpcID,
		},
		Zone: &vpcv1.ZoneIdentity{
			Name: &IBMCloudProps.Zone,
		},
	})
	log.Infof("Creating subnet %s in VPC %s in Zone %s.\n", IBMCloudProps.SubnetName, IBMCloudProps.VpcID, IBMCloudProps.Zone)
	subnet, _, err := IBMCloudProps.VPC.CreateSubnet(options)
	if err != nil {
		return err
	}
	IBMCloudProps.SubnetID = *subnet.ID
	log.Infof("Created subnet with ID %s.\n", IBMCloudProps.SubnetID)

	if len(IBMCloudProps.SubnetID) <= 0 {
		return errors.New("SubnetID is empty, unknown error happened when create Subnet.")
	}

	return nil
}

func deleteSubnet() error {
	if !IBMCloudProps.IsProvNewSubnet {
		fmt.Printf("Do not delete because using existing Subnet: %s\n", IBMCloudProps.SubnetID)
		return nil
	}

	options := &vpcv1.DeleteSubnetOptions{}
	options.SetID(IBMCloudProps.SubnetID)
	log.Infof("Deleting subnet with ID %s.\n", IBMCloudProps.SubnetID)
	_, err := IBMCloudProps.VPC.DeleteSubnet(options)

	if err != nil {
		return err
	}
	log.Infof("Deleted subnet with ID %s.\n", IBMCloudProps.SubnetID)
	return nil
}

func createVpcImpl() error {
	err := createVPC()
	if err != nil {
		return err
	}
	log.Info("waiting for the VPC to be available before creating subnet...")
	// wait vpc ready before create subnet
	time.Sleep(60 * time.Second)
	return createSubnet()
}

func deleteVpcImpl() error {
	err := deleteSubnet()
	if err != nil {
		return err
	}
	return deleteVPC()
}

func isClusterReady(clrName string) (bool, error) {
	target := containerv2.ClusterTargetHeader{
		Provider: "vpc-gen2",
	}
	clusters, err := IBMCloudProps.ClusterAPI.List(target)
	if err != nil {
		return false, err
	}
	for _, cluster := range clusters {
		if cluster.Name == clrName && strings.EqualFold(cluster.State, "normal") {
			return true, nil
		}
	}
	return false, nil
}

func foundCluster(clrName string) (bool, error) {
	target := containerv2.ClusterTargetHeader{
		Provider: "vpc-gen2",
	}
	clusters, err := IBMCloudProps.ClusterAPI.List(target)
	if err != nil {
		return false, err
	}
	for _, cluster := range clusters {
		if cluster.Name == clrName {
			return true, nil
		}
	}
	return false, nil
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
	log.Trace("CreateCluster()")

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
	log.Infof("Creating cluster %s.\n", IBMCloudProps.ClusterName)
	_, err := IBMCloudProps.ClusterAPI.Create(clusterInfo, target)
	if err != nil {
		return err
	}

	clusterReady := false
	waitMinutes := 50
	log.Infof("Waiting for cluster %s to be available.\n", IBMCloudProps.ClusterName)
	for i := 0; i <= waitMinutes; i++ {
		ready, err := isClusterReady(IBMCloudProps.ClusterName)
		if err != nil {
			log.Warnf("Err %s happened when retrieve cluster, try again...\n", err)
			continue
		}
		if ready {
			log.Infof("Cluster %s is available.\n", IBMCloudProps.ClusterName)
			clusterReady = true
			break
		}
		log.Infof("Waited %d minutes...\n", i)

		time.Sleep(60 * time.Second)
	}

	if !clusterReady {
		return fmt.Errorf("Cluster %s was created but not ready in %d minutes.\n", IBMCloudProps.ClusterName, waitMinutes)
	}
	return nil
}

func (p *IBMCloudProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("CreateVPC()")
	return createVpcImpl()
}

func (p *IBMCloudProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("DeleteCluster()")

	target := containerv2.ClusterTargetHeader{}
	log.Infof("Deleting Cluster %s.\n", IBMCloudProps.ClusterName)
	err := IBMCloudProps.ClusterAPI.Delete(IBMCloudProps.ClusterName, target)
	if err != nil {
		return err
	}

	clusterRemoved := false
	waitMinutes := 50
	log.Infof("Waiting for cluster %s to be removed...\n", IBMCloudProps.ClusterName)
	for i := 0; i <= waitMinutes; i++ {
		found, err := foundCluster(IBMCloudProps.ClusterName)
		if err != nil {
			log.Warnf("Err %s happened when retrieve cluster, try again...\n", err)
			continue
		}
		if !found {
			log.Infof("Cluster %s is removed.\n", IBMCloudProps.ClusterName)
			clusterRemoved = true
			break
		}
		log.Infof("Waited %d minutes...\n", i)
		time.Sleep(60 * time.Second)
	}

	if !clusterRemoved {
		return fmt.Errorf("Cluster %s was not removed completely in %d minutes.\n", IBMCloudProps.ClusterName, waitMinutes)
	}
	return nil
}

func (p *IBMCloudProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("DeleteVPC()")
	return deleteVpcImpl()
}

func (p *IBMCloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	log.Trace("UploadPodvm()")

	filePath, err := filepath.Abs(imagePath)
	if err != nil {
		return err
	}

	conf := aws.NewConfig().
		WithEndpoint(IBMCloudProps.CosServiceURL).
		WithCredentials(ibmiam.NewStaticCredentials(aws.NewConfig(),
			IBMCloudProps.IamServiceURL, IBMCloudProps.CosApiKey, IBMCloudProps.CosInstanceID)).
		WithS3ForcePathStyle(true)

	sess := cosession.Must(cosession.NewSession(conf))
	log.Info("session initialized.")

	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	log.Infof("qcow2 image file %s validated.\n", imagePath)

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
	log.Infof("\nFile %s uploaded to bucket.\n", key)

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
	imageName := strings.TrimSuffix(key, filepath.Ext(key))
	options := &vpcv1.CreateImageOptions{}
	options.SetImagePrototype(&vpcv1.ImagePrototype{
		Name: &imageName,
		File: &vpcv1.ImageFilePrototype{
			Href: &cosID,
		},
		OperatingSystem: operatingSystemIdentityModel,
	})
	log.Infof("cosID %s, imageName %s.\n", cosID, imageName)
	image, _, err := IBMCloudProps.VPC.CreateImage(options)
	if err != nil {
		return err
	}
	IBMCloudProps.PodvmImageID = *image.ID
	log.Infof("Image %s with PodvmImageID %s created from the bucket.\n", key, IBMCloudProps.PodvmImageID)

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
