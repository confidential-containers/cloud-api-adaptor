// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"crypto/sha256"

	"sigs.k8s.io/e2e-framework/pkg/envconf"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/utils"

	"github.com/IBM-Cloud/bluemix-go/api/container/containerv1"
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
	foundVPC, err := findVPC(IBMCloudProps.VpcName)
	if err != nil {
		return err
	}
	if foundVPC != nil {
		IBMCloudProps.VpcID = *foundVPC.ID
		log.Infof("VPC %s with ID %s exists already", IBMCloudProps.VpcName, IBMCloudProps.VpcID)
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
		log.Infof("Creating VPC %s in ResourceGroupID %s.", IBMCloudProps.VpcName, IBMCloudProps.ResourceGroupID)
		vpcInstance, _, err := IBMCloudProps.VPC.CreateVPC(options)
		if err != nil {
			return err
		}

		IBMCloudProps.VpcID = *vpcInstance.ID
		log.Infof("Created VPC with ID %s in ResourceGroupID %s.", IBMCloudProps.VpcID, IBMCloudProps.ResourceGroupID)

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
	log.Infof("Got VPC default SecurityGroupID %s.", IBMCloudProps.SecurityGroupID)

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
	log.Infof("Deleting VPC with ID %s.", IBMCloudProps.VpcID)
	_, err = IBMCloudProps.VPC.DeleteVPC(deleteVpcOptions)

	if err != nil {
		return err
	}
	log.Infof("Deleted VPC with ID %s.", IBMCloudProps.VpcID)
	return nil
}

func createPublicGateway(subnetID, vpcID string) error {
	vpcIDentityModel := &vpcv1.VPCIdentityByID{
		ID: &vpcID,
	}
	zoneIdentityModel := &vpcv1.ZoneIdentityByName{
		Name: &IBMCloudProps.Zone,
	}

	createPublicGatewayOptions := IBMCloudProps.VPC.NewCreatePublicGatewayOptions(
		vpcIDentityModel,
		zoneIdentityModel,
	)
	createPublicGatewayOptions.SetName(IBMCloudProps.PublicGatewayName)
	log.Infof("Creating Public Gateway %s.", IBMCloudProps.PublicGatewayName)
	publicGateway, _, err := IBMCloudProps.VPC.CreatePublicGateway(createPublicGatewayOptions)
	if err != nil {
		return err
	}
	IBMCloudProps.PublicGatewayID = *publicGateway.ID
	log.Infof("Created Public Gateway with ID %s.", IBMCloudProps.PublicGatewayID)

	options := &vpcv1.SetSubnetPublicGatewayOptions{}
	options.SetID(subnetID)
	options.SetPublicGatewayIdentity(&vpcv1.PublicGatewayIdentity{
		ID: &IBMCloudProps.PublicGatewayID,
	})
	_, _, err = IBMCloudProps.VPC.SetSubnetPublicGateway(options)
	if err != nil {
		return err
	}
	log.Infof("Attached Public Gateway %s to Subnet %s.", IBMCloudProps.PublicGatewayID, subnetID)
	return nil
}

func deletePublicGateway(subnetID, publicGatewayID string) error {
	unsetOptions := IBMCloudProps.VPC.NewUnsetSubnetPublicGatewayOptions(
		subnetID,
	)

	_, err := IBMCloudProps.VPC.UnsetSubnetPublicGateway(unsetOptions)
	if err != nil {
		return err
	}
	log.Infof("Public Gateway %s was unattached from Subnet %s.", publicGatewayID, subnetID)

	deleteOptions := &vpcv1.DeletePublicGatewayOptions{}
	deleteOptions.SetID(publicGatewayID)
	_, err = IBMCloudProps.VPC.DeletePublicGateway(deleteOptions)
	if err != nil {
		return err
	}
	log.Infof("Public Gateway %s was deleted.", publicGatewayID)

	return nil
}

func getSubnetPublicGateway(subnetID string) (*vpcv1.PublicGateway, error) {
	options := &vpcv1.GetSubnetPublicGatewayOptions{}
	options.SetID(subnetID)
	publicGateway, _, err := IBMCloudProps.VPC.GetSubnetPublicGateway(options)
	if err != nil {
		return nil, err
	}
	return publicGateway, nil
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
	} else {
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
		log.Infof("Creating subnet %s in VPC %s in Zone %s.", IBMCloudProps.SubnetName, IBMCloudProps.VpcID, IBMCloudProps.Zone)
		subnet, _, err := IBMCloudProps.VPC.CreateSubnet(options)
		if err != nil {
			return err
		}
		IBMCloudProps.SubnetID = *subnet.ID
		log.Infof("Created subnet with ID %s.", IBMCloudProps.SubnetID)
	}

	if len(IBMCloudProps.SubnetID) <= 0 {
		return errors.New("SubnetID is empty, unknown error happened when create Subnet.")
	}

	gateway, _ := getSubnetPublicGateway(IBMCloudProps.SubnetID)
	if gateway != nil {
		IBMCloudProps.PublicGatewayID = *gateway.ID
		log.Infof("PublicGateway %s exists in subnet.", IBMCloudProps.PublicGatewayID)
	} else {
		err := createPublicGateway(IBMCloudProps.SubnetID, IBMCloudProps.VpcID)
		if err != nil {
			return err
		}
	}

	if len(IBMCloudProps.PublicGatewayID) <= 0 {
		return errors.New("PublicGatewayID is empty, unknown error happened when create PublicGateway in Subnet.")
	}

	return nil
}

// TODO, nice to have retry if SDK client did not do that for well known http errors
func findAttachedLoadBalancer(subnetName string) (*vpcv1.LoadBalancer, error) {
	options := &vpcv1.ListLoadBalancersOptions{}

	pager, err := IBMCloudProps.VPC.NewLoadBalancersPager(options)
	if err != nil {
		return nil, err
	}

	var allResults []vpcv1.LoadBalancer
	for pager.HasNext() {
		nextPage, err := pager.GetNext()
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, nextPage...)
	}

	for _, loadBalancer := range allResults {
		log.Tracef("Checking loadBalancer %s.", *loadBalancer.Name)
		for _, subnet := range loadBalancer.Subnets {
			if *subnet.Name == subnetName {
				return &loadBalancer, nil
			}
		}
	}
	return nil, nil
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

	gateway, _ := getSubnetPublicGateway(IBMCloudProps.SubnetID)
	if gateway != nil { // ignore error when getting gateway
		err := deletePublicGateway(IBMCloudProps.SubnetID, *gateway.ID)
		if err != nil {
			log.Warnf("Failed to delete PublicGateway %s.", *gateway.ID)
			return err
		}
	}

	// Waiting the attached Load Balancers to be removed
	waitMinutes := 5
	log.Infof("Waiting for attached LoadBalancers to be removed from %s ...", IBMCloudProps.SubnetName)
	for i := 0; i <= waitMinutes; i++ {
		foundlb, _ := findAttachedLoadBalancer(IBMCloudProps.SubnetName)
		if foundlb == nil {
			log.Infof("All attached LoadBalancers are removed from %s ...", IBMCloudProps.SubnetName)
			break
		}
		log.Infof("Waiting for attached LoadBalancer %s to be removed.", *foundlb.Name)
		log.Infof("Waited %d minutes...", i)
		time.Sleep(60 * time.Second)
	}

	options := &vpcv1.DeleteSubnetOptions{}
	options.SetID(IBMCloudProps.SubnetID)
	log.Infof("Deleting subnet with ID %s.", IBMCloudProps.SubnetID)
	_, err = IBMCloudProps.VPC.DeleteSubnet(options)

	if err != nil {
		return err
	}
	log.Infof("Waiting for Subnet  %s to be removed...", IBMCloudProps.SubnetName)
	for i := 0; i <= waitMinutes; i++ {
		foundsb, _ := findSubnet(IBMCloudProps.SubnetName)
		if foundsb == nil {
			log.Infof("Subnet %s is removed  ...", IBMCloudProps.SubnetName)
			break
		}
		log.Infof("Waiting for %s to be removed.", *foundsb.Name)
		log.Infof("Waited %d minutes...", i)
		time.Sleep(60 * time.Second)
	}
	log.Infof("Deleted subnet with ID %s.", IBMCloudProps.SubnetID)
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
	waitMinutes := 5
	log.Infof("Waiting for SSH key  %s to be removed...", IBMCloudProps.SshKeyID)
	for i := 0; i <= waitMinutes; i++ {
		foundSsh, err := findSshKey(IBMCloudProps.SshKeyID)
		if foundSsh == nil {
			log.Infof("SSH Key %s with ID %s is deleted.", IBMCloudProps.SshKeyName, IBMCloudProps.SshKeyID)
			return nil
		}
		if err != nil {
			return err
		}
		log.Infof("Waiting for SSH %s to be removed.", *foundSsh.Name)
		log.Infof("Waited %d minutes...", i)
		time.Sleep(60 * time.Second)
	}
	return errors.New("Failed to Delete SSH Key")
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

func getKubeconfig(kubecfgDir string) (*containerv1.ClusterKeyInfo, error) {
	if err := os.MkdirAll(kubecfgDir, 0755); err != nil {
		return nil, err
	}
	target := containerv2.ClusterTargetHeader{
		Provider: "vpc-gen2",
	}
	kubeCfgInfo, err := IBMCloudProps.ClusterAPI.GetClusterConfigDetail(IBMCloudProps.ClusterName, kubecfgDir, true, target)
	log.Infof("Downloaded cluster %s kube configuration into folder %s", IBMCloudProps.ClusterName, kubecfgDir)
	log.Debugf("%+v", kubeCfgInfo)
	if err != nil {
		return nil, err
	}

	return &kubeCfgInfo, nil
}

// IBMCloudProvisioner implements the CloudProvisioner interface for ibmcloud.
type IBMCloudProvisioner struct {
}

func NewIBMCloudProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	if err := InitIBMCloudProperties(properties); err != nil {
		return nil, err
	}

	if IBMCloudProps.IsSelfManaged {
		return &IBMSelfManagedClusterProvisioner{}, nil
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
		log.Infof("Cluster %s. exists already.", IBMCloudProps.ClusterName)
	} else {
		clusterInfo := containerv2.ClusterCreateRequest{
			DisablePublicServiceEndpoint: false,
			Name:                         IBMCloudProps.ClusterName,
			Provider:                     "vpc-gen2",
			KubeVersion:                  IBMCloudProps.KubeVersion,
			WorkerPools: containerv2.WorkerPoolConfig{
				CommonWorkerPoolConfig: containerv2.CommonWorkerPoolConfig{
					DiskEncryption:  true,
					Flavor:          IBMCloudProps.WorkerFlavor,
					OperatingSystem: IBMCloudProps.WorkerOS,
					VpcID:           IBMCloudProps.VpcID,
					WorkerCount:     IBMCloudProps.WorkerCount,
					Zones: []containerv2.Zone{
						{
							ID:       IBMCloudProps.Zone,
							SubnetID: IBMCloudProps.SubnetID,
						},
					},
					Labels: map[string]string{
						"node.kubernetes.io/worker": "",
					},
				},
			},
		}
		target := containerv2.ClusterTargetHeader{}
		log.Infof("Creating cluster %s.", IBMCloudProps.ClusterName)
		_, err := IBMCloudProps.ClusterAPI.Create(clusterInfo, target)
		if err != nil {
			return err
		}
	}

	clusterReady := false
	waitMinutes := 50
	log.Infof("Waiting for cluster %s to be available.", IBMCloudProps.ClusterName)
	for i := 0; i <= waitMinutes; i++ {
		ready, err := isClusterReady(IBMCloudProps.ClusterName)
		if err != nil {
			log.Warnf("Err %s happened when retrieve cluster, try again...", err)
			continue
		}
		if ready {
			log.Infof("Cluster %s is available.", IBMCloudProps.ClusterName)
			clusterReady = true
			break
		}
		log.Infof("Waited %d minutes...", i)

		time.Sleep(60 * time.Second)
	}

	if !clusterReady {
		return fmt.Errorf("Cluster %s was created but not ready in %d minutes.", IBMCloudProps.ClusterName, waitMinutes)
	}

	home, _ := os.UserHomeDir()
	kubeconfigDir := path.Join(home, ".kube")
	log.Infof("Sync cluster kubeconfig to %s with current config context", kubeconfigDir)
	kubeConfigInfo, err := getKubeconfig(kubeconfigDir)
	if err != nil {
		return err
	}
	cfg.WithKubeconfigFile(kubeConfigInfo.FilePath)

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
		log.Infof("Cluster %s. does not exist.", IBMCloudProps.ClusterName)
		return nil
	}

	target := containerv2.ClusterTargetHeader{}
	log.Infof("Deleting Cluster %s.", IBMCloudProps.ClusterName)
	err = IBMCloudProps.ClusterAPI.Delete(IBMCloudProps.ClusterName, target)
	if err != nil {
		return err
	}

	clusterRemoved := false
	waitMinutes := 50
	log.Infof("Waiting for cluster %s to be removed...", IBMCloudProps.ClusterName)
	for i := 0; i <= waitMinutes; i++ {
		foundClr, err := findCluster(IBMCloudProps.ClusterName)
		if err != nil {
			log.Warnf("Err %s happened when retrieve cluster, try again...", err)
			continue
		}
		if foundClr == nil {
			log.Infof("Cluster %s is removed.", IBMCloudProps.ClusterName)
			clusterRemoved = true
			break
		}
		log.Infof("Waited %d minutes...", i)
		time.Sleep(60 * time.Second)
	}

	if !clusterRemoved {
		return fmt.Errorf("Cluster %s was not removed completely in %d minutes.", IBMCloudProps.ClusterName, waitMinutes)
	}
	return nil
}

func (p *IBMCloudProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("DeleteVPC()")
	return deleteVpcImpl()
}

// TODO, nice to have retry if SDK client did not do that for well known http errors
func findImage(imageName string) (*vpcv1.Image, error) {
	listImagesOptions := &vpcv1.ListImagesOptions{}
	listImagesOptions.SetVisibility("private")

	pager, err := IBMCloudProps.VPC.NewImagesPager(listImagesOptions)
	if err != nil {
		return nil, err
	}

	var allResults []vpcv1.Image
	for pager.HasNext() {
		nextPage, err := pager.GetNext()
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, nextPage...)
	}
	for _, image := range allResults {
		log.Tracef("Checking Image %s.", *image.Name)
		if *image.Name == imageName {
			return &image, nil
		}
	}
	return nil, nil
}

func getSha256sum(imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	sum := hash.Sum(nil)
	return fmt.Sprintf("%x", sum), nil
}

func (p *IBMCloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	log.Trace("UploadPodvm()")

	if len(IBMCloudProps.ApiKey) <= 0 {
		return errors.New("APIKEY must be set to upload podvm image")
	}

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
	log.Info("cos session initialized.")

	newImageSha256sum, err := getSha256sum(imagePath)

	if err != nil {
		return err
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	log.Infof("qcow2 image file %s validated, sha256sum: %s.", imagePath, newImageSha256sum)

	key := filepath.Base(filePath)
	imageName := strings.TrimSuffix(key, filepath.Ext(key))

	existImage, err := findImage(imageName)
	if err != nil {
		return err
	}
	if existImage != nil {
		if newImageSha256sum == *existImage.File.Checksums.Sha256 {
			IBMCloudProps.PodvmImageID = *existImage.ID
			log.Infof("Found exist image %s with same content, PodvmImageID %s, sha256sum: %s.", key, IBMCloudProps.PodvmImageID, *existImage.File.Checksums.Sha256)
			return nil
		} else {
			log.Infof("Found exist image %s, sha256sum: %s with old content deleting it ...", imageName, *existImage.File.Checksums.Sha256)
			deleteOptions := &vpcv1.DeleteImageOptions{}
			deleteOptions.SetID(*existImage.ID)
			_, err := IBMCloudProps.VPC.DeleteImage(deleteOptions)
			if err != nil {
				return err
			}
			// Waiting the old image to be removed
			waitMinutes := 5
			log.Infof("Waiting for exist image with PodvmImageID %s to be removed.", *existImage.ID)
			for i := 0; i <= waitMinutes; i++ {
				foundImage, _ := findImage(imageName)
				if foundImage == nil {
					log.Infof("The exist image with PodvmImageID %s is removed.", *existImage.ID)
					break
				}
				log.Infof("Waited %d minutes...", i)
				time.Sleep(60 * time.Second)
			}
		}
	}

	hide_progress := os.Getenv("HIDE_UPLOADER_PROGRESS")

	reader := &utils.CustomReader{
		Fp:           file,
		Size:         fileInfo.Size(),
		SignMap:      map[int64]struct{}{},
		HideProgress: hide_progress,
	}

	uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
		u.PartSize = 5 * 1024 * 1024
		u.LeavePartsOnError = true
	})

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(IBMCloudProps.Bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		return err
	}
	log.Infof("File %s uploaded to bucket.", key)

	var osNames []string
	if strings.EqualFold("s390x", IBMCloudProps.PodvmImageArch) {
		if strings.Contains(IBMCloudProps.InstanceProfile, "e-") {
			osNames = []string{"hyper-protect-1-0-s390x"}
		} else {
			osNames = []string{"ubuntu-22-04-s390x"}
		}
	} else {
		osNames = []string{"ubuntu-24-04-amd64"}

	}
	operatingSystemIdentityModel := &vpcv1.OperatingSystemIdentityByName{
		Name: &osNames[0],
	}

	cosID := "cos://" + IBMCloudProps.Region + "/" + IBMCloudProps.Bucket + "/" + key

	options := &vpcv1.CreateImageOptions{}
	options.SetImagePrototype(&vpcv1.ImagePrototype{
		Name: &imageName,
		File: &vpcv1.ImageFilePrototype{
			Href: &cosID,
		},
		OperatingSystem: operatingSystemIdentityModel,
	})
	log.Infof("cosID %s, imageName %s.", cosID, imageName)
	image, _, err := IBMCloudProps.VPC.CreateImage(options)
	if err != nil {
		return err
	}
	IBMCloudProps.PodvmImageID = *image.ID
	log.Infof("Image %s with PodvmImageID %s created from the bucket.", imageName, IBMCloudProps.PodvmImageID)
	return nil
}

func getProfileList() string {
	var profileList string
	if strings.EqualFold("s390x", IBMCloudProps.PodvmImageArch) {
		if strings.Contains(IBMCloudProps.InstanceProfile, "e-") {
			profileList = "bz2e-2x8,cz2e-2x4,mz2e-2x16"
		} else {
			profileList = "bz2-2x8,cz2-2x4,mz2-2x16"
		}
	} else {
		if strings.Contains(IBMCloudProps.InstanceProfile, "dc-") {
			profileList = "bx3dc-2x10,cx3dc-2x5,bx3dc-4x20" // Profiles with confidential computing support
		} else {
			profileList = "bx2-2x8,cx2-2x4,mx2-2x16"
		}
	}
	return profileList
}

func (p *IBMCloudProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"CLOUD_PROVIDER":                       IBMCloudProps.IBMCloudProvider,
		"IBMCLOUD_VPC_ENDPOINT":                IBMCloudProps.VpcServiceURL,
		"IBMCLOUD_RESOURCE_GROUP_ID":           IBMCloudProps.ResourceGroupID,
		"IBMCLOUD_SSH_KEY_ID":                  IBMCloudProps.SshKeyID,
		"IBMCLOUD_PODVM_IMAGE_ID":              IBMCloudProps.PodvmImageID,
		"IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME": IBMCloudProps.InstanceProfile,
		"IBMCLOUD_ZONE":                        IBMCloudProps.Zone,
		"IBMCLOUD_VPC_SUBNET_ID":               IBMCloudProps.SubnetID,
		"IBMCLOUD_VPC_SG_ID":                   IBMCloudProps.SecurityGroupID,
		"IBMCLOUD_VPC_ID":                      IBMCloudProps.VpcID,
		"CRI_RUNTIME_ENDPOINT":                 "/run/cri-runtime/containerd.sock",
		"IBMCLOUD_API_KEY":                     IBMCloudProps.ApiKey,
		"IBMCLOUD_IAM_PROFILE_ID":              IBMCloudProps.IamProfileID,
		"IBMCLOUD_IAM_ENDPOINT":                IBMCloudProps.IamServiceURL,
		"IBMCLOUD_PODVM_INSTANCE_PROFILE_LIST": getProfileList(),
		"TUNNEL_TYPE":                          IBMCloudProps.TunnelType,
		"VXLAN_PORT":                           IBMCloudProps.VxlanPort,
		"DISABLECVM":                           strconv.FormatBool(IBMCloudProps.DisableCVM),
		"INITDATA":                             IBMCloudProps.InitData,
		"IBMCLOUD_CLUSTER_ID":                  IBMCloudProps.ClusterID,
		"TAGS":                                 IBMCloudProps.Tags,
		"CONTAINER_RUNTIME":                    IBMCloudProps.ContainerRuntime,
	}
}

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
