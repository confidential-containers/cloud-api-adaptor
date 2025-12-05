// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kconf "sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	EksVersion         = "1.34"
	AwsCredentialsFile = "aws-cred.env"
)

var AWSProps = &AWSProvisioner{}

// S3Bucket represents an S3 bucket where the podvm image should be uploaded
type S3Bucket struct {
	Client *s3.Client
	Name   string // Bucket name
	Key    string // Object key
	Region string // AWS region
}

// AMIImage represents an AMI image
type AMIImage struct {
	BaseName        string
	Client          *ec2.Client
	Description     string // Image description
	DiskDescription string // Disk description
	DiskFormat      string // Disk format
	EBSSnapshotId   string // EBS disk snapshot ID
	ID              string // AMI image ID
	RootDeviceName  string // Root device name
	VmImportRole    string // vmimport role name
	BootUefi        bool   // If true, enable UEFI boot mode (required for AMD SEV-SNP)
}

// Vpc represents an AWS VPC
type Vpc struct {
	BaseName          string
	CidrBlock         string
	Client            *ec2.Client
	ID                string
	InternetGatewayId string
	Region            string
	RouteTableId      string
	SecurityGroupId   string
	SubnetId          string
	SecondarySubnetId string
}

// Cluster defines create/delete/access interfaces to Kubernetes clusters
type Cluster interface {
	CreateCluster() error               // Create the Kubernetes cluster
	DeleteCluster() error               // Delete the Kubernetes cluster
	GetKubeconfigFile() (string, error) // Get the path to the kubeconfig file
}

// EKSCluster represents an EKS cluster
type EKSCluster struct {
	AwsConfig  aws.Config
	Name       string
	NumWorkers int32
	SshKpName  string
	Version    string
	Vpc        *Vpc
}

// OnPremCluster represents an existing and running cluster
type OnPremCluster struct {
}

// AWSProvisioner implements the CloudProvision interface.
type AWSProvisioner struct {
	AwsConfig         aws.Config
	iamClient         *iam.Client
	containerRuntime  string // Name of the container runtime
	Cluster           Cluster
	Disablecvm        string
	ec2Client         *ec2.Client
	s3Client          *s3.Client
	Bucket            *S3Bucket
	PauseImage        string
	Image             *AMIImage
	Vpc               *Vpc
	PodvmInstanceType string
	PublicIP          string
	TunnelType        string
	VxlanPort         string
	SshKpName         string
}

// AwsInstallOverlay implements the InstallOverlay interface
type AwsInstallOverlay struct {
	Overlay *pv.KustomizeOverlay
}

// NewAWSProvisioner instantiates the AWS provisioner
func NewAWSProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	var cluster Cluster

	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Error("Failed to load AWS config")
		return nil, err
	}

	if properties["aws_region"] != "" {
		cfg.Region = properties["aws_region"]
	} else {
		properties["aws_region"] = cfg.Region
	}

	ec2Client := ec2.NewFromConfig(cfg)

	if properties["resources_basename"] == "" {
		properties["resources_basename"] = "caa-e2e-test-" + strconv.FormatInt(time.Now().Unix(), 10)
	}

	if properties["podvm_aws_instance_type"] == "" {
		properties["podvm_aws_instance_type"] = "t2.medium"
	}

	vpc := NewVpc(ec2Client, properties)

	if properties["cluster_type"] == "" ||
		properties["cluster_type"] == "onprem" {
		cluster = NewOnPremCluster()
		// The podvm should be created with public IP so CAA can connect
		properties["use_public_ip"] = "true"
	} else if properties["cluster_type"] == "eks" {
		cluster = NewEKSCluster(cfg, vpc, properties["ssh_kp_name"], properties["eks_name"])
	} else {
		return nil, fmt.Errorf("Cluster type '%s' not implemented",
			properties["cluster_type"])
	}

	AWSProps = &AWSProvisioner{
		AwsConfig: cfg,
		iamClient: iam.NewFromConfig(cfg),
		ec2Client: ec2.NewFromConfig(cfg),
		s3Client:  s3.NewFromConfig(cfg),
		Bucket: &S3Bucket{
			Client: s3.NewFromConfig(cfg),
			Name:   properties["resources_basename"] + "-bucket",
			Key:    "", // To be defined when the file is uploaded
			Region: cfg.Region,
		},
		containerRuntime:  properties["container_runtime"],
		Cluster:           cluster,
		Image:             NewAMIImage(ec2Client, properties),
		Disablecvm:        properties["disablecvm"],
		PauseImage:        properties["pause_image"],
		Vpc:               vpc,
		PodvmInstanceType: properties["podvm_aws_instance_type"],
		PublicIP:          properties["use_public_ip"],
		TunnelType:        properties["tunnel_type"],
		VxlanPort:         properties["vxlan_port"],
		SshKpName:         properties["ssh_kp_name"],
	}

	return AWSProps, nil
}

func (a *AWSProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	err := a.Cluster.CreateCluster()
	if err != nil {
		return err
	}

	kubeconfigPath, err := a.Cluster.GetKubeconfigFile()
	if err != nil {
		return err
	}
	*cfg = *envconf.NewWithKubeConfig(kubeconfigPath)

	return nil
}

func (a *AWSProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	var err error

	if a.Vpc.ID == "" {
		log.Infof("Create AWS VPC on region %s", a.AwsConfig.Region)
		if err = a.Vpc.createVpc(); err != nil {
			return err
		}
		log.Infof("VPC Id: %s", a.Vpc.ID)
	}

	if a.Vpc.SubnetId == "" {
		log.Infof("Create subnet on VPC %s", a.Vpc.ID)
		if err = a.Vpc.createSubnet(); err != nil {
			return err
		}
		log.Infof("Subnet Id: %s", a.Vpc.SubnetId)

		if err = a.Vpc.setupVpcNetworking(); err != nil {
			return err
		}
	}

	if a.Vpc.SecurityGroupId == "" {
		log.Infof("Create security group on VPC %s", a.Vpc.ID)
		if err = a.Vpc.setupSecurityGroup(); err != nil {
			return err
		}
		log.Infof("Security groupd Id: %s", a.Vpc.SecurityGroupId)
	}

	return nil
}

func (aws *AWSProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return aws.Cluster.DeleteCluster()
}

func (a *AWSProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	var err error
	vpc := a.Vpc

	if vpc.SubnetId != "" {
		log.Infof("Delete subnet: %s", vpc.SubnetId)
		if err = vpc.deleteSubnet(); err != nil {
			return err
		}
	}

	if vpc.SecurityGroupId != "" {
		log.Infof("Delete security group: %s", vpc.SecurityGroupId)
		if err = vpc.deleteSecurityGroup(); err != nil {
			return err
		}
	}

	if vpc.ID != "" {
		log.Infof("Delete vpc: %s", vpc.ID)
		if err = vpc.deleteVpc(); err != nil {
			return err
		}
	}

	if a.Image.ID != "" || a.Image.EBSSnapshotId != "" {
		if err = a.Image.deregisterImage(); err != nil {
			return err
		}
	}

	// Delete the vmimport role if it exists
	_, err = a.iamClient.GetRole(context.TODO(), &iam.GetRoleInput{
		RoleName: aws.String(a.Image.VmImportRole),
	})
	if err == nil {
		log.Infof("Delete VM import role: %s", a.Image.VmImportRole)
		// First delete the role policy
		_, err = a.iamClient.DeleteRolePolicy(context.TODO(), &iam.DeleteRolePolicyInput{
			RoleName:   aws.String(a.Image.VmImportRole),
			PolicyName: aws.String("vmimport"),
		})
		if err != nil {
			log.Errorf("Failed to delete VM import role policy: %s", err)
		}

		// Then delete the role
		_, err = a.iamClient.DeleteRole(context.TODO(), &iam.DeleteRoleInput{
			RoleName: aws.String(a.Image.VmImportRole),
		})
		if err != nil {
			log.Errorf("Failed to delete VM import role: %s", err)
		}
	}

	if a.Bucket.Key != "" {
		log.Infof("Delete key %s from bucket: %s", a.Bucket.Key, a.Bucket.Name)
		if err = a.Bucket.deleteKey(); err != nil {
			return err
		}
	}

	// Delete the S3 bucket
	log.Infof("Delete S3 bucket: %s", a.Bucket.Name)
	if err = a.Bucket.deleteBucket(); err != nil {
		log.Errorf("Failed to delete S3 bucket: %s", err)
	}

	return nil
}

func (a *AWSProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	credentials, _ := a.AwsConfig.Credentials.Retrieve(context.TODO())

	return map[string]string{
		"CONTAINER_RUNTIME":       a.containerRuntime,
		"disablecvm":              a.Disablecvm,
		"pause_image":             a.PauseImage,
		"podvm_launchtemplate":    "",
		"podvm_ami":               a.Image.ID,
		"podvm_aws_instance_type": a.PodvmInstanceType,
		"sg_ids":                  a.Vpc.SecurityGroupId, // TODO: what other SG needed?
		"subnet_id":               a.Vpc.SubnetId,
		"ssh_kp_name":             a.SshKpName,
		"region":                  a.AwsConfig.Region,
		"resources_basename":      a.Vpc.BaseName,
		"access_key_id":           credentials.AccessKeyID,
		"secret_access_key":       credentials.SecretAccessKey,
		"session_token":           credentials.SessionToken,
		"use_public_ip":           a.PublicIP,
		"tunnel_type":             a.TunnelType,
		"vxlan_port":              a.VxlanPort,
	}
}

func (a *AWSProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	// AWS EC2 image-import does not support qcow2 files so convert the image to raw format.
	imageRawFile, err := os.CreateTemp("", "podvm.*.raw")
	imageRawPath := imageRawFile.Name()
	imageRawFile.Close()
	if err != nil {
		return err
	}
	defer func() {
		_, err := os.Stat(imageRawPath)
		if err == nil {
			os.Remove(imageRawPath)
		}
	}()

	log.Infof("Convert qcow2 image to raw")
	if err = ConvertQcow2ToRaw(imagePath, imageRawPath); err != nil {
		return err
	}

	// Create the S3 bucket
	log.Infof("Create bucket '%s'", a.Bucket.Name)
	if err = a.Bucket.createBucket(); err != nil {
		return err
	}

	// Create the vmimport role
	log.Infof("Create vmimport service role: %s", a.Image.VmImportRole)
	if err = createVmimportServiceRole(ctx, a.iamClient, a.Bucket.Name, a.Image.VmImportRole); err != nil {
		return err
	}

	// Upload raw image to S3
	log.Infof("Upload image %s to S3 bucket '%s'", imageRawPath, a.Bucket.Name)
	if err = a.Bucket.uploadLargeFileWithCli(imageRawPath); err != nil {
		return err
	}

	log.Infof("Import disk snapshot for S3 key '%s'", a.Bucket.Key)
	if err = a.Image.importEBSSnapshot(a.Bucket); err != nil {
		return err
	}

	imageNameSuffix := "-" + strconv.FormatInt(time.Now().Unix(), 10)
	imageName := strings.Replace(filepath.Base(imagePath), ".qcow2", imageNameSuffix, 1)
	log.Infof("Register image with name: %s", imageName)
	err = a.Image.registerImage(imageName)
	if err != nil {
		return err
	}
	log.Infof("New AMI ID: %s", a.Image.ID)
	return nil
}

func NewVpc(client *ec2.Client, properties map[string]string) *Vpc {
	// Initialize the VPC CidrBlock
	cidrBlock := properties["aws_vpc_cidrblock"]
	if cidrBlock == "" {
		cidrBlock = "10.0.0.0/24"
	}

	return &Vpc{
		BaseName:          properties["resources_basename"],
		CidrBlock:         cidrBlock,
		Client:            client,
		ID:                properties["aws_vpc_id"],
		Region:            properties["aws_region"],
		SecurityGroupId:   properties["aws_vpc_sg_id"],
		SubnetId:          properties["aws_vpc_subnet_id"],
		InternetGatewayId: properties["aws_vpc_igw_id"],
		RouteTableId:      properties["aws_vpc_rt_id"],
	}
}

// createVpc creates the VPC
func (v *Vpc) createVpc() error {
	vpc, err := v.Client.CreateVpc(context.TODO(), &ec2.CreateVpcInput{
		CidrBlock:         aws.String(v.CidrBlock),
		TagSpecifications: defaultTagSpecifications(v.BaseName+"-vpc", ec2types.ResourceTypeVpc),
	})
	if err != nil {
		return err
	}

	v.ID = *vpc.Vpc.VpcId
	return nil
}

// createSubnet creates the VPC subnet
func (v *Vpc) createSubnet() error {
	subnet, err := v.Client.CreateSubnet(context.TODO(),
		&ec2.CreateSubnetInput{
			VpcId:             aws.String(v.ID),
			CidrBlock:         aws.String("10.0.0.0/25"),
			TagSpecifications: defaultTagSpecifications(v.BaseName+"-subnet", ec2types.ResourceTypeSubnet),
		})

	if err != nil {
		return err
	}

	v.SubnetId = *subnet.Subnet.SubnetId

	// Allow for instances created on the subnet to have a public IP assigned
	if _, err = v.Client.ModifySubnetAttribute(context.TODO(),
		&ec2.ModifySubnetAttributeInput{
			MapPublicIpOnLaunch: &ec2types.AttributeBooleanValue{
				Value: aws.Bool(true),
			},
			SubnetId: aws.String(v.SubnetId),
		}); err != nil {
		return err
	}

	return nil
}

func (v *Vpc) createSecondarySubnet() error {
	if v.SecondarySubnetId != "" {
		return nil
	}

	// EKS requires at least two subnets and they should be on different
	// Availability zones on the same region. So let's ensure this secondary
	// subnet's AZ don't clash with the primary's.
	subnets, err := v.Client.DescribeSubnets(context.TODO(),
		&ec2.DescribeSubnetsInput{
			SubnetIds: []string{v.SubnetId},
		})
	if err != nil {
		return err
	}

	primarySubnetAz := *subnets.Subnets[0].AvailabilityZone
	secondarySubnetAz := v.Region + "a"
	if secondarySubnetAz == primarySubnetAz {
		secondarySubnetAz = v.Region + "b"
	}

	subnet, err := v.Client.CreateSubnet(context.TODO(),
		&ec2.CreateSubnetInput{
			AvailabilityZone:  aws.String(secondarySubnetAz),
			VpcId:             aws.String(v.ID),
			CidrBlock:         aws.String("10.0.0.128/25"),
			TagSpecifications: defaultTagSpecifications(v.BaseName+"-subnet-2", ec2types.ResourceTypeSubnet),
		})

	if err != nil {
		return err
	}

	v.SecondarySubnetId = *subnet.Subnet.SubnetId

	return nil
}

// setupInternetGateway creates an internet gateway and table of routes, and
// associated them with the VPC
func (v *Vpc) setupVpcNetworking() error {
	var (
		rtOutput  *ec2.CreateRouteTableOutput
		igwOutput *ec2.CreateInternetGatewayOutput
		err       error
	)

	if v.SubnetId == "" {
		return fmt.Errorf("Missing subnet Id to setup the VPC %s network\n", v.ID)
	}

	if igwOutput, err = v.Client.CreateInternetGateway(context.TODO(),
		&ec2.CreateInternetGatewayInput{
			TagSpecifications: defaultTagSpecifications(v.BaseName+"-igw", ec2types.ResourceTypeInternetGateway),
		}); err != nil {
		return err
	}
	v.InternetGatewayId = *igwOutput.InternetGateway.InternetGatewayId

	if _, err = v.Client.AttachInternetGateway(context.TODO(),
		&ec2.AttachInternetGatewayInput{
			InternetGatewayId: igwOutput.InternetGateway.InternetGatewayId,
			VpcId:             aws.String(v.ID),
		}); err != nil {
		return err
	}

	if rtOutput, err = v.Client.CreateRouteTable(context.TODO(),
		&ec2.CreateRouteTableInput{
			VpcId:             aws.String(v.ID),
			TagSpecifications: defaultTagSpecifications(v.BaseName+"-rtb", ec2types.ResourceTypeRouteTable),
		}); err != nil {
		return err
	}

	if _, err := v.Client.CreateRoute(context.TODO(),
		&ec2.CreateRouteInput{
			RouteTableId:         rtOutput.RouteTable.RouteTableId,
			DestinationCidrBlock: aws.String("0.0.0.0/0"),
			GatewayId:            igwOutput.InternetGateway.InternetGatewayId,
		}); err != nil {
		return err
	}

	v.RouteTableId = *rtOutput.RouteTable.RouteTableId

	if _, err := v.Client.AssociateRouteTable(context.TODO(),
		&ec2.AssociateRouteTableInput{
			RouteTableId: rtOutput.RouteTable.RouteTableId,
			SubnetId:     aws.String(v.SubnetId),
		}); err != nil {
		return err
	}

	return nil
}

func (v *Vpc) setupSecurityGroup() error {
	if sgOutput, err := v.Client.CreateSecurityGroup(context.TODO(),
		&ec2.CreateSecurityGroupInput{
			Description:       aws.String("cloud-api-adaptor e2e tests"),
			GroupName:         aws.String(v.BaseName + "-sg"),
			VpcId:             aws.String(v.ID),
			TagSpecifications: defaultTagSpecifications(v.BaseName+"-sg", ec2types.ResourceTypeSecurityGroup),
		}); err != nil {
		return err
	} else {
		v.SecurityGroupId = *sgOutput.GroupId
	}

	if _, err := v.Client.AuthorizeSecurityGroupIngress(context.TODO(),
		&ec2.AuthorizeSecurityGroupIngressInput{
			IpPermissions: []ec2types.IpPermission{
				{
					FromPort:   aws.Int32(-1),
					IpProtocol: aws.String("icmp"),
					IpRanges: []ec2types.IpRange{
						{
							CidrIp:      aws.String("0.0.0.0/0"),
							Description: aws.String("ingress rule for icmp access"),
						},
					},
					ToPort: aws.Int32(-1),
				},
				{
					FromPort:   aws.Int32(22),
					IpProtocol: aws.String("tcp"),
					IpRanges: []ec2types.IpRange{
						{
							CidrIp:      aws.String("0.0.0.0/0"),
							Description: aws.String("ingress rule for ssh access"),
						},
					},
					ToPort: aws.Int32(22),
				},
				{
					FromPort:   aws.Int32(6443),
					IpProtocol: aws.String("tcp"),
					IpRanges: []ec2types.IpRange{
						{
							CidrIp:      aws.String("0.0.0.0/0"),
							Description: aws.String("ingress rule for https traffic"),
						},
					},
					ToPort: aws.Int32(6443),
				},
				{
					FromPort:   aws.Int32(15150),
					IpProtocol: aws.String("tcp"),
					IpRanges: []ec2types.IpRange{
						{
							CidrIp:      aws.String("0.0.0.0/0"),
							Description: aws.String("ingress rule for CAA proxy traffic"),
						},
					},
					ToPort: aws.Int32(15150),
				},
			},
			GroupId: aws.String(v.SecurityGroupId),
		}); err != nil {
		return err
	}

	if _, err := v.Client.AuthorizeSecurityGroupEgress(context.TODO(),
		&ec2.AuthorizeSecurityGroupEgressInput{
			IpPermissions: []ec2types.IpPermission{
				{
					FromPort:   aws.Int32(6443),
					IpProtocol: aws.String("tcp"),
					IpRanges: []ec2types.IpRange{
						{
							CidrIp:      aws.String("0.0.0.0/0"),
							Description: aws.String("egress rule for https traffic"),
						},
					},
					ToPort: aws.Int32(6443),
				},
			},
			GroupId: aws.String(v.SecurityGroupId),
		}); err != nil {
		return err
	}

	return nil
}

// deleteSecurityGroup deletes the security group.
func (v *Vpc) deleteSecurityGroup() error {
	if v.SecurityGroupId == "" {
		return nil
	}

	if _, err := v.Client.DeleteSecurityGroup(context.TODO(),
		&ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(v.SecurityGroupId),
		}); err != nil {
		return err
	}

	return nil
}

// deleteSubnet deletes the subnet. Instances running on the subnet will
// be terminated before.
func (v *Vpc) deleteSubnet() error {
	if v.SubnetId == "" {
		return nil
	}

	// There will be needed to terminate all instances launched on this subnet
	// before the attempt to delete the subnet.

	describeInstances, err := v.Client.DescribeInstances(context.TODO(),
		&ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{
					Name:   aws.String("subnet-id"),
					Values: []string{v.SubnetId},
				},
			},
		})
	if err != nil {
		return err
	}

	// Getting the instances IDs
	instanceIds := make([]string, 0)
	for _, reservation := range describeInstances.Reservations {
		for _, instance := range reservation.Instances {
			instanceIds = append(instanceIds, *instance.InstanceId)
		}
	}

	if len(instanceIds) > 0 {
		// Delete all instances in a single step
		if _, err = v.Client.TerminateInstances(context.TODO(),
			&ec2.TerminateInstancesInput{
				InstanceIds: instanceIds,
			}); err != nil {
			return err
		}
		// Wait them to terminate
		waiter := ec2.NewInstanceTerminatedWaiter(v.Client)
		if err = waiter.Wait(context.TODO(), &ec2.DescribeInstancesInput{
			InstanceIds: instanceIds,
		}, time.Minute*5); err != nil {
			return err
		}
	}

	// Finally delete the subnet
	if _, err = v.Client.DeleteSubnet(context.TODO(),
		&ec2.DeleteSubnetInput{
			SubnetId: aws.String(v.SubnetId),
		}); err != nil {
		return err
	}

	return nil
}

// deleteVpc deletes the VPC. All resources attached to it will be
// deleted before.
func (v *Vpc) deleteVpc() error {
	var err error

	if v.ID == "" {
		return nil
	}

	// Delete the networking resources first
	if v.RouteTableId != "" {
		if _, err = v.Client.DeleteRouteTable(context.TODO(),
			&ec2.DeleteRouteTableInput{
				RouteTableId: aws.String(v.RouteTableId),
			}); err != nil {
			return err
		}
	}

	// The internet gateway time
	if v.InternetGatewayId != "" {
		if _, err = v.Client.DetachInternetGateway(context.TODO(),
			&ec2.DetachInternetGatewayInput{
				InternetGatewayId: aws.String(v.InternetGatewayId),
				VpcId:             aws.String(v.ID),
			}); err != nil {
			return err
		}
		if _, err = v.Client.DeleteInternetGateway(context.TODO(),
			&ec2.DeleteInternetGatewayInput{
				InternetGatewayId: aws.String(v.InternetGatewayId),
			}); err != nil {
			return err
		}
	}

	// Then finally the VPC itself
	if _, err := v.Client.DeleteVpc(context.TODO(),
		&ec2.DeleteVpcInput{
			VpcId: aws.String(v.ID),
		}); err != nil {
		return err
	}

	return nil
}

// bucketExists checks if the S3 bucket exists
func (b *S3Bucket) bucketExists() (bool, error) {
	listBucketsResult, err := b.Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return false, err
	}

	// Check if our bucket exists in the list
	for _, bucket := range listBucketsResult.Buckets {
		if *bucket.Name == b.Name {
			return true, nil
		}
	}

	return false, nil
}

// createBucket Creates the S3 bucket
func (b *S3Bucket) createBucket() error {
	exists, err := b.bucketExists()
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	createBucketInput := &s3.CreateBucketInput{
		Bucket: &b.Name,
	}

	// For regions other than us-east-1, we need to specify a location constraint
	if b.Region != "" && b.Region != "us-east-1" {
		createBucketInput.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(b.Region),
		}
	}

	_, err = b.Client.CreateBucket(context.TODO(), createBucketInput)
	if err != nil {
		return err
	}

	// Set the bucket policy
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Sid": "AllowVMIE",
			"Effect": "Allow",
			"Principal": { "Service": "vmie.amazonaws.com" },
			"Action": ["s3:GetBucketLocation", "s3:GetObject", "s3:ListBucket" ],
			"Resource": ["arn:aws:s3:::%s", "arn:aws:s3:::%s/*"]}]
	}`, b.Name, b.Name)

	if _, err = b.Client.PutBucketPolicy(context.TODO(), &s3.PutBucketPolicyInput{
		Bucket: &b.Name,
		Policy: &policy,
	}); err != nil {
		return err
	}

	return nil
}

// deleteBucket deletes the S3 bucket and all its contents
func (b *S3Bucket) deleteBucket() error {
	exists, err := b.bucketExists()
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	// List all objects in the bucket
	listResult, err := b.Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: &b.Name,
	})
	if err != nil {
		return err
	}

	// Delete all objects if any exist
	if len(listResult.Contents) > 0 {
		var objectsToDelete []s3types.ObjectIdentifier
		for _, obj := range listResult.Contents {
			objectsToDelete = append(objectsToDelete, s3types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		_, err = b.Client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
			Bucket: &b.Name,
			Delete: &s3types.Delete{
				Objects: objectsToDelete,
			},
		})
		if err != nil {
			log.Warnf("Failed to delete objects from bucket %s: %v", b.Name, err)
		}
	}

	// Delete the bucket
	_, err = b.Client.DeleteBucket(context.TODO(), &s3.DeleteBucketInput{
		Bucket: &b.Name,
	})
	if err != nil {
		return err
	}

	return nil
}

// createVmimportServiceRole Creates the vmimport service role as required to use the VM snaphot import feature.
//
//	For further details see https://docs.aws.amazon.com/vm-import/latest/userguide/required-permissions.html
func createVmimportServiceRole(ctx context.Context, client *iam.Client, bucketName string, roleName string) error {
	_, err := client.GetRole(context.TODO(), &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err == nil {
		// The role exists, do nothing
		return nil
	}

	// Create the service role
	trustPolicy := `{
		"Version":"2012-10-17",
		"Statement":[
			{
				"Effect":"Allow",
				"Principal":{ "Service":"vmie.amazonaws.com" },
				"Action": "sts:AssumeRole",
				"Condition":{"StringEquals":{"sts:Externalid":"vmimport"}}
			}
		]
	}`

	if _, err = client.CreateRole(context.TODO(), &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(trustPolicy),
		RoleName:                 aws.String(roleName),
	}); err != nil {
		return err
	}

	// Set the role policy
	rolePolicy := fmt.Sprintf(`{
		"Version":"2012-10-17",
		"Statement":[
			{
				"Effect":"Allow",
				"Action":["s3:GetBucketLocation","s3:GetObject","s3:ListBucket"],
				"Resource":["arn:aws:s3:::%s","arn:aws:s3:::%s/*"]
			},
			{
				"Effect":"Allow",
				"Action":["ec2:ModifySnapshotAttribute","ec2:CopySnapshot","ec2:RegisterImage","ec2:Describe*"],
				"Resource":"*"
			}
		]
	}`, bucketName, bucketName)

	if _, err = client.PutRolePolicy(context.TODO(), &iam.PutRolePolicyInput{
		PolicyDocument: aws.String(rolePolicy),
		PolicyName:     aws.String("vmimport"),
		RoleName:       aws.String(roleName),
	}); err != nil {
		return err
	}

	return nil
}

func NewAMIImage(client *ec2.Client, properties map[string]string) *AMIImage {
	// If disablecvm is empty or false then it wants confidential VM and
	// for AMD SEV-SNP it needs to enable UEFI boot.
	bootUefi := false
	disablecvm := properties["disablecvm"]
	if disablecvm == "" || disablecvm == "false" {
		bootUefi = true
	}

	return &AMIImage{
		BaseName:        properties["resources_basename"],
		Client:          client,
		Description:     "Peer Pod VM image",
		DiskDescription: "Peer Pod VM disk",
		DiskFormat:      "RAW",
		EBSSnapshotId:   "", // To be defined when the snapshot is created
		ID:              properties["podvm_aws_ami_id"],
		RootDeviceName:  "/dev/xvda",
		VmImportRole:    properties["resources_basename"] + "-vmimport",
		BootUefi:        bootUefi,
	}
}

// importEBSSnapshot Imports the disk image into the EBS
func (i *AMIImage) importEBSSnapshot(bucket *S3Bucket) error {
	// Create the import snapshot task
	importSnapshotOutput, err := i.Client.ImportSnapshot(context.TODO(), &ec2.ImportSnapshotInput{
		Description: aws.String("Peer Pod VM disk snapshot"),
		DiskContainer: &ec2types.SnapshotDiskContainer{
			Description: aws.String(i.DiskDescription),
			Format:      aws.String(i.DiskFormat),
			UserBucket: &ec2types.UserBucket{
				S3Bucket: aws.String(bucket.Name),
				S3Key:    aws.String(bucket.Key),
			},
		},
		RoleName:          aws.String(i.VmImportRole),
		TagSpecifications: defaultTagSpecifications(i.BaseName+"-snap", ec2types.ResourceTypeImportSnapshotTask),
	})
	if err != nil {
		return err
	}

	//taskId := *importSnapshotOutput.ImportTaskId
	describeTasksInput := &ec2.DescribeImportSnapshotTasksInput{
		ImportTaskIds: []string{*importSnapshotOutput.ImportTaskId},
	}

	// Wait the import task to finish
	waiter := ec2.NewSnapshotImportedWaiter(i.Client)
	if err = waiter.Wait(context.TODO(), describeTasksInput, time.Minute*10); err != nil {
		return err
	}

	// Finally get the snapshot ID
	describeTasks, err := i.Client.DescribeImportSnapshotTasks(context.TODO(), describeTasksInput)
	if err != nil {
		return err
	}
	taskDetail := describeTasks.ImportSnapshotTasks[0].SnapshotTaskDetail
	i.EBSSnapshotId = *taskDetail.SnapshotId

	// Let's warn but ignore any tagging error
	if _, err = i.Client.CreateTags(context.TODO(), &ec2.CreateTagsInput{
		Resources: []string{i.EBSSnapshotId},
		Tags: []ec2types.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(i.BaseName + "-snap"),
			},
		},
	}); err != nil {
		log.Warnf("Failed to tag EBS snapshot %s: %v", i.EBSSnapshotId, err)
	}

	return nil
}

// registerImage Registers an AMI image
func (i *AMIImage) registerImage(imageName string) error {

	if i.EBSSnapshotId == "" {
		return fmt.Errorf("EBS Snapshot ID not found\n")
	}

	registerInput := &ec2.RegisterImageInput{
		Name:         aws.String(imageName),
		Architecture: ec2types.ArchitectureValuesX8664,
		BlockDeviceMappings: []ec2types.BlockDeviceMapping{{
			DeviceName: aws.String(i.RootDeviceName),
			Ebs: &ec2types.EbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				SnapshotId:          aws.String(i.EBSSnapshotId),
			},
		}},
		Description:        aws.String(i.Description),
		EnaSupport:         aws.Bool(true),
		RootDeviceName:     aws.String(i.RootDeviceName),
		VirtualizationType: aws.String("hvm"),
		TagSpecifications:  defaultTagSpecifications(i.BaseName+"-img", ec2types.ResourceTypeImage),
	}

	// If BootUefi is true, enable UEFI boot mode for AMD SEV-SNP
	if i.BootUefi {
		registerInput.BootMode = ec2types.BootModeValuesUefi
	}

	result, err := i.Client.RegisterImage(context.TODO(), registerInput)
	if err != nil {
		return err
	}

	// Save the AMI ID
	i.ID = *result.ImageId
	return nil
}

// deregisterImage Deregisters an AMI image. The associated EBS snapshot is deleted too.
func (i *AMIImage) deregisterImage() error {
	var err error

	if i.ID != "" {
		log.Infof("Deregister AMI ID: %s", i.ID)
		_, err = i.Client.DeregisterImage(context.TODO(), &ec2.DeregisterImageInput{
			ImageId: aws.String(i.ID),
		})
		if err != nil {
			log.Errorf("Failed to deregister AMI: %s", err)
		}
	}

	// Removing the EBS snapshot
	if i.EBSSnapshotId != "" {
		log.Infof("Delete Snapshot ID: %s", i.EBSSnapshotId)
		_, err = i.Client.DeleteSnapshot(context.TODO(), &ec2.DeleteSnapshotInput{
			SnapshotId: aws.String(i.EBSSnapshotId),
		})
		if err != nil {
			log.Errorf("Failed to delete snapshot: %s", err)
		}
	}

	return err
}

// uploadLargeFileWithCli Uploads large files (>5GB) using the AWS CLI
func (b *S3Bucket) uploadLargeFileWithCli(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	key := stat.Name()
	defer func() {
		if err == nil {
			b.Key = key
		}
	}()

	s3uri := "s3://" + b.Name + "/" + key

	// TODO: region!
	cmd := exec.Command("aws", "s3", "cp", "--no-progress", filepath, s3uri)
	out, err := cmd.CombinedOutput()
	fmt.Printf("%s\n", out)
	if err != nil {
		return err
	}

	return nil
}

func (b *S3Bucket) deleteKey() error {
	if _, err := b.Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(b.Name),
		Key:    aws.String(b.Key),
	}); err != nil {
		return err
	}

	return nil
}

// ConvertQcow2ToRaw Converts an qcow2 image to raw. Requires `qemu-img` installed.
func ConvertQcow2ToRaw(qcow2 string, raw string) error {
	cmd := exec.Command("qemu-img", "convert", "-O", "raw", qcow2, raw)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// createCredentialFile Creates the AWS credential file in the install overlay directory
// that's used by kustomize the setup CAA. The session_token parameter is optional.
func createCredentialFile(dir, access_key_id, secret_access_key, session_token string) error {
	content := fmt.Sprintf("AWS_ACCESS_KEY_ID=%s\nAWS_SECRET_ACCESS_KEY=%s\n", access_key_id, secret_access_key)
	if session_token != "" {
		content += fmt.Sprintf("AWS_SESSION_TOKEN=%s\n", session_token)
	}
	err := os.WriteFile(filepath.Join(dir, AwsCredentialsFile), []byte(content), 0666)
	if err != nil {
		return nil
	}

	return nil
}

func NewAwsInstallOverlay(installDir, provider string) (pv.InstallOverlay, error) {
	overlayDir := filepath.Join(installDir, "overlays", provider)

	// The credential file should exist in the overlay directory otherwise kustomize fails
	// to load it. At this point we don't know the key id nor access key, so using empty
	// values (later the file will be re-written properly).
	err := createCredentialFile(overlayDir, "", "", "")
	if err != nil {
		return nil, err
	}

	overlay, err := pv.NewKustomizeOverlay(overlayDir)
	if err != nil {
		return nil, err
	}

	return &AwsInstallOverlay{
		Overlay: overlay,
	}, nil
}

func (a *AwsInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return a.Overlay.Apply(ctx, cfg)
}

func (a *AwsInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return a.Overlay.Delete(ctx, cfg)
}

func (a *AwsInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	var err error

	// Mapping the internal properties to ConfigMapGenerator properties.
	mapProps := map[string]string{
		"disablecvm":              "DISABLECVM",
		"pause_image":             "PAUSE_IMAGE",
		"podvm_launchtemplate":    "PODVM_LAUNCHTEMPLATE_NAME",
		"podvm_ami":               "PODVM_AMI_ID",
		"podvm_aws_instance_type": "PODVM_INSTANCE_TYPE",
		"sg_ids":                  "AWS_SG_IDS",
		"subnet_id":               "AWS_SUBNET_ID",
		"ssh_kp_name":             "SSH_KP_NAME",
		"region":                  "AWS_REGION",
		"tunnel_type":             "TUNNEL_TYPE",
		"vxlan_port":              "VXLAN_PORT",
		"use_public_ip":           "USE_PUBLIC_IP",
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			if err = a.Overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm",
				v, properties[k]); err != nil {
				return err
			}
		}
	}

	if properties["access_key_id"] != "" && properties["secret_access_key"] != "" {
		if err = createCredentialFile(a.Overlay.ConfigDir, properties["access_key_id"], properties["secret_access_key"], properties["session_token"]); err != nil {
			return err
		}

		if err = a.Overlay.SetKustomizeSecretGeneratorEnv("peer-pods-secret", AwsCredentialsFile); err != nil {
			return err
		}
	}

	if err = a.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}

// NewEKSCluster instantiates a new EKS Cluster struct.
// It requires a AWS configuration with access and authentication information, a
// VPC already instantiated and with a public subnet, and an EC2 SSH key-pair used
// to access the cluster's worker nodes.
// If eksName is provided, it will use an existing cluster with that name instead of creating a new one
// otherwise it will create a cluster and set a default name.
func NewEKSCluster(cfg aws.Config, vpc *Vpc, SshKpName string, eksName string) *EKSCluster {

	return &EKSCluster{
		AwsConfig:  cfg,
		Name:       eksName,
		NumWorkers: 1,
		SshKpName:  SshKpName,
		Version:    EksVersion,
		Vpc:        vpc,
	}
}

// CreateCluster creates a new EKS cluster using eksctl.
// EKS should be created with at least two subnets so a secondary will be
// created if it does not exist on the VPC already.
func (e *EKSCluster) CreateCluster() error {
	var err error

	if e.Name != "" {
		log.Infof("Using existing EKS cluster: %s", e.Name)
		return nil
	}

	e.Name = e.Vpc.BaseName + "-k8s"

	if e.Vpc.SecondarySubnetId == "" {
		log.Info("Create a secondary subnet for EKS")
		if err = e.Vpc.createSecondarySubnet(); err != nil {
			return err
		}
		log.Infof("Secondary subnet Id: %s", e.Vpc.SecondarySubnetId)
	}

	log.Infof("Creating the EKS cluster: %s", e.Name)

	cmdArgs := []string{
		"create", "cluster",
		"--name", e.Name,
		"--version", e.Version,
		"--region", e.AwsConfig.Region,
		"--kubeconfig", e.Name + "-kubeconfig",
		"--vpc-private-subnets", e.Vpc.SubnetId + "," + e.Vpc.SecondarySubnetId,
		"--nodegroup-name", e.Name,
		"--nodes", strconv.FormatInt(int64(e.NumWorkers), 10),
		"--nodes-min", strconv.FormatInt(int64(e.NumWorkers), 10),
		"--nodes-max", strconv.FormatInt(int64(e.NumWorkers), 10),
		"--node-type", "t3.medium",
		"--node-ami-family", "Ubuntu2404",
		"--ssh-access",
		"--ssh-public-key", e.SshKpName,
		"--node-private-networking",
		"--with-oidc",
	}

	cmd := exec.Command("eksctl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("failed to create EKS cluster: %w", err)
	}

	// TODO: This block copy most of the `AddNodeRoleWorkerLabel()` code. We
	// refactor that function to be usable here too.
	kubeconfigPath, err := e.GetKubeconfigFile()
	if err != nil {
		return err
	}
	cfg := envconf.NewWithKubeConfig(kubeconfigPath)

	client, err := cfg.NewClient()
	if err != nil {
		return err
	}
	nodelist := &corev1.NodeList{}
	if err := client.Resources().List(context.TODO(), nodelist); err != nil {
		return err
	}
	// Use full path to avoid overwriting other labels (see RFC 6902)
	payload := []pv.PatchLabel{{
		Op: "add",
		// "/" must be written as ~1 (see RFC 6901)
		Path:  "/metadata/labels/node.kubernetes.io~1worker",
		Value: "",
	}}
	payloadBytes, _ := json.Marshal(payload)
	for _, node := range nodelist.Items {
		if err := client.Resources().Patch(context.TODO(), &node, k8s.Patch{PatchType: types.JSONPatchType, Data: payloadBytes}); err != nil {
			return err
		}
	}

	return nil
}

// DeleteCluster deletes the EKS cluster using eksctl.
func (e *EKSCluster) DeleteCluster() error {
	if e.Name == "" {
		return nil
	}
	log.Infof("Deleting the EKS cluster: %s using eksctl...", e.Name)

	cmdArgs := []string{
		"delete", "cluster",
		"--name", e.Name,
		"--region", e.AwsConfig.Region,
		"--wait",
		"--timeout", "15m",
	}

	cmd := exec.Command("eksctl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %w", err)
	}

	return nil
}

// GetKubeconfigFile returns a kubeconfig for the EKS cluster
func (e *EKSCluster) GetKubeconfigFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	targetDir := filepath.Join(homeDir, ".kube", e.Name)
	if err = os.MkdirAll(targetDir, 0750); err != nil {
		return "", err
	}
	targetFile := filepath.Join(targetDir, "config")

	// Use eksctl to write kubeconfig
	cmdArgs := []string{
		"utils", "write-kubeconfig",
		"--cluster", e.Name,
		"--region", e.AwsConfig.Region,
		"--kubeconfig", targetFile,
	}

	cmd := exec.Command("eksctl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return targetFile, nil
}

func NewOnPremCluster() *OnPremCluster {
	return &OnPremCluster{}
}

// CreateCluster does nothing as the cluster should exist already.
func (o *OnPremCluster) CreateCluster() error {
	log.Info("On-prem cluster type selected. Nothing to do.")

	return nil
}

// DeleteCluster does nothing.
func (o *OnPremCluster) DeleteCluster() error {
	log.Info("On-prem cluster type selected. Nothing to do.")

	return nil
}

// GetKubeconfigFile looks for the kubeconfig on the default locations
func (o *OnPremCluster) GetKubeconfigFile() (string, error) {
	kubeconfigPath := kconf.ResolveKubeConfigFile()
	if kubeconfigPath == "" {
		return "", fmt.Errorf("Unabled to find a kubeconfig file")
	}

	return kubeconfigPath, nil
}
