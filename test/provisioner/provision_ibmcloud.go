//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/envconf"

	"github.com/confidential-containers/cloud-api-adaptor/test/utils"

	"github.com/IBM-Cloud/bluemix-go/api/container/containerv2"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	cosession "github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3/s3manager"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	log "github.com/sirupsen/logrus"
)

func init() {
	newProvisionerFunctions["ibmcloud"] = NewIBMCloudProvisioner
}

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
	foundVPC, err := findVPC(IBMCloudProps.VpcName)
	if err != nil {
		return err
	}
	if foundVPC != nil {
		IBMCloudProps.VpcID = *foundVPC.ID
		log.Infof("VPC %s with ID %s exists alread", IBMCloudProps.VpcName, IBMCloudProps.VpcID)
	} else {
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
	foundVPC, err := findVPC(IBMCloudProps.VpcName)
	if err != nil {
		return err
	}
	if foundVPC == nil {
		log.Infof("VPC %s does not exist.", IBMCloudProps.VpcName)
		return nil
	}

	IBMCloudProps.VpcID = *foundVPC.ID
	log.Infof("Found VPC %s with ID %s.", IBMCloudProps.VpcName, IBMCloudProps.VpcID)

	deleteVpcOptions := &vpcv1.DeleteVPCOptions{}
	deleteVpcOptions.SetID(IBMCloudProps.VpcID)
	log.Infof("Deleting VPC with ID %s.\n", IBMCloudProps.VpcID)
	_, err = IBMCloudProps.VPC.DeleteVPC(deleteVpcOptions)

	if err != nil {
		return err
	}
	log.Infof("Deleted VPC with ID %s.\n", IBMCloudProps.VpcID)
	return nil
}

func createSubnet() error {
	log.Trace("createSubnet()")
	foundSubnet, err := findSubnet(IBMCloudProps.SubnetName)
	if err != nil {
		return err
	}
	if foundSubnet != nil {
		IBMCloudProps.SubnetID = *foundSubnet.ID
		log.Infof("Subnet %s with ID %s exists already.", IBMCloudProps.SubnetName, IBMCloudProps.SubnetID)
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
	foundSubnet, err := findSubnet(IBMCloudProps.SubnetName)
	if err != nil {
		return err
	}
	if foundSubnet == nil {
		log.Infof("Subnet %s does not exist.", IBMCloudProps.SubnetName)
		return nil
	}

	IBMCloudProps.SubnetID = *foundSubnet.ID
	log.Infof("Found subnet %s with ID %s.", IBMCloudProps.SubnetName, IBMCloudProps.SubnetID)

	options := &vpcv1.DeleteSubnetOptions{}
	options.SetID(IBMCloudProps.SubnetID)
	log.Infof("Deleting subnet with ID %s.\n", IBMCloudProps.SubnetID)
	_, err = IBMCloudProps.VPC.DeleteSubnet(options)

	if err != nil {
		return err
	}
	log.Infof("Deleted subnet with ID %s.\n", IBMCloudProps.SubnetID)
	return nil
}

func createVpcImpl() error {
	err := createSshKey()
	if err != nil {
		return err
	}

	err = createVPC()
	if err != nil {
		return err
	}
	log.Trace("Waiting for the VPC to be available before creating subnet...")

	return createSubnet()
}

func deleteVpcImpl() error {
	err := deleteSshKey()
	if err != nil {
		return err
	}

	err = deleteSubnet()
	if err != nil {
		return err
	}
	return deleteVPC()
}

func createSshKey() error {
	key, err := findSshKey(IBMCloudProps.SshKeyName)
	if err != nil {
		return err
	}
	if key != nil {
		IBMCloudProps.SshKeyID = *key.ID
		log.Infof("SSH Key %s with ID %s exists already, we can just use it.", IBMCloudProps.SshKeyName, IBMCloudProps.SshKeyID)
		return nil
	}

	options := &vpcv1.CreateKeyOptions{}
	options.SetName(IBMCloudProps.SshKeyName)
	options.SetPublicKey(IBMCloudProps.SshKeyContent)
	key, _, err = IBMCloudProps.VPC.CreateKey(options)

	if err != nil {
		return err
	}

	IBMCloudProps.SshKeyID = *key.ID
	log.Infof("SSH Key %s with ID %s is created.", IBMCloudProps.SshKeyName, IBMCloudProps.SshKeyID)
	return nil
}

func deleteSshKey() error {
	key, err := findSshKey(IBMCloudProps.SshKeyName)
	if err != nil {
		return err
	}
	if key == nil {
		log.Infof("SSH Key %s does not exist.", IBMCloudProps.SshKeyName)
		return nil
	}

	IBMCloudProps.SshKeyID = *key.ID

	deleteKeyOptions := &vpcv1.DeleteKeyOptions{}
	deleteKeyOptions.SetID(IBMCloudProps.SshKeyID)
	_, err = IBMCloudProps.VPC.DeleteKey(deleteKeyOptions)
	if err != nil {
		return err
	}
	log.Infof("SSH Key %s with ID %s is deleted.", IBMCloudProps.SshKeyName, IBMCloudProps.SshKeyID)
	return nil
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

// TODO, nice to have retry if SDK client did not do that for well known http errors
func findCluster(clrName string) (*containerv2.ClusterInfo, error) {
	target := containerv2.ClusterTargetHeader{
		Provider: "vpc-gen2",
	}
	clusters, err := IBMCloudProps.ClusterAPI.List(target)
	if err != nil {
		return nil, err
	}
	for _, cluster := range clusters {
		if cluster.Name == clrName {
			return &cluster, nil
		}
	}
	return nil, nil
}

// TODO, nice to have retry if SDK client did not do that for well known http errors
func findVPC(vpcName string) (*vpcv1.VPC, error) {
	listVpcsOptions := &vpcv1.ListVpcsOptions{}

	pager, err := IBMCloudProps.VPC.NewVpcsPager(listVpcsOptions)
	if err != nil {
		return nil, err
	}

	var allResults []vpcv1.VPC
	for pager.HasNext() {
		nextPage, err := pager.GetNext()
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, nextPage...)
	}
	for _, vpc := range allResults {
		log.Tracef("Checking vpc %s.", *vpc.Name)
		if *vpc.Name == vpcName {
			return &vpc, nil
		}
	}
	return nil, nil
}

// TODO, nice to have retry if SDK client did not do that for well known http errors
func findSubnet(subnetName string) (*vpcv1.Subnet, error) {
	listSubnetsOptions := &vpcv1.ListSubnetsOptions{}

	pager, err := IBMCloudProps.VPC.NewSubnetsPager(listSubnetsOptions)
	if err != nil {
		return nil, err
	}

	var allResults []vpcv1.Subnet
	for pager.HasNext() {
		nextPage, err := pager.GetNext()
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, nextPage...)
	}
	for _, subnet := range allResults {
		log.Tracef("Checking subnet %s.", *subnet.Name)
		if *subnet.Name == subnetName {
			return &subnet, nil
		}
	}
	return nil, nil
}

func findSshKey(keyName string) (*vpcv1.Key, error) {
	listKeysOptions := &vpcv1.ListKeysOptions{}

	pager, err := IBMCloudProps.VPC.NewKeysPager(listKeysOptions)
	if err != nil {
		return nil, err
	}

	var allResults []vpcv1.Key
	for pager.HasNext() {
		nextPage, err := pager.GetNext()
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, nextPage...)
	}
	for _, key := range allResults {
		log.Tracef("Checking SSH Key %s.", *key.Name)
		if *key.Name == keyName {
			return &key, nil
		}
	}
	return nil, nil
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

	foundClr, err := findCluster(IBMCloudProps.ClusterName)
	if err != nil {
		return err
	}
	if foundClr != nil {
		log.Infof("Cluster %s. exists already.\n", IBMCloudProps.ClusterName)
	} else {
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

	foundClr, err := findCluster(IBMCloudProps.ClusterName)
	if err != nil {
		return err
	}
	if foundClr == nil {
		log.Infof("Cluster %s. does not exist.\n", IBMCloudProps.ClusterName)
		return nil
	}

	target := containerv2.ClusterTargetHeader{}
	log.Infof("Deleting Cluster %s.\n", IBMCloudProps.ClusterName)
	err = IBMCloudProps.ClusterAPI.Delete(IBMCloudProps.ClusterName, target)
	if err != nil {
		return err
	}

	clusterRemoved := false
	waitMinutes := 50
	log.Infof("Waiting for cluster %s to be removed...\n", IBMCloudProps.ClusterName)
	for i := 0; i <= waitMinutes; i++ {
		foundClr, err := findCluster(IBMCloudProps.ClusterName)
		if err != nil {
			log.Warnf("Err %s happened when retrieve cluster, try again...\n", err)
			continue
		}
		if foundClr == nil {
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
