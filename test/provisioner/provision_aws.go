//go:build aws

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	log "github.com/sirupsen/logrus"
	kconf "sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func init() {
	// Add this implementation to the list of provisioners.
	newProvisionerFunctions["aws"] = NewAWSProvisioner
	newInstallOverlayFunctions["aws"] = NewAwsInstallOverlay
}

// S3Bucket represents an S3 bucket where the podvm image should be uploaded
type S3Bucket struct {
	Client *s3.Client
	Name   string // Bucket name
	Key    string // Object key
}

// AMIImage represents an AMI image
type AMIImage struct {
	Client          *ec2.Client
	Description     string // Image description
	DiskDescription string // Disk description
	DiskFormat      string // Disk format
	EBSSnapshotId   string // EBS disk snapshot ID
	ID              string // AMI image ID
	RootDeviceName  string // Root device name
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

// AWSProvisioner implements the CloudProvision interface.
type AWSProvisioner struct {
	AwsConfig  aws.Config
	iamClient  *iam.Client
	ec2Client  *ec2.Client
	s3Client   *s3.Client
	Bucket     *S3Bucket
	PauseImage string
	Image      *AMIImage
	Vpc        *Vpc
	VxlanPort  string
	SshKpName  string
}

// AwsInstallOverlay implements the InstallOverlay interface
type AwsInstallOverlay struct {
	overlay *KustomizeOverlay
}

// NewAWSProvisioner instantiates the AWS provisioner
func NewAWSProvisioner(properties map[string]string) (CloudProvisioner, error) {
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
	return &AWSProvisioner{
		AwsConfig: cfg,
		iamClient: iam.NewFromConfig(cfg),
		ec2Client: ec2.NewFromConfig(cfg),
		s3Client:  s3.NewFromConfig(cfg),
		Bucket: &S3Bucket{
			Client: s3.NewFromConfig(cfg),
			Name:   "peer-pods-tests",
			Key:    "", // To be defined when the file is uploaded
		},
		Image:      NewAMIImage(ec2Client, properties),
		PauseImage: properties["pause_image"],
		Vpc:        NewVpc(ec2Client, properties),
		VxlanPort:  properties["vxlan_port"],
		SshKpName:  properties["ssh_kp_name"],
	}, nil
}

func (a *AWSProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	kubeconfigPath := kconf.ResolveKubeConfigFile()
	if kubeconfigPath == "" {
		return fmt.Errorf("Unabled to find a kubeconfig file")
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
	return nil
}

func (a *AWSProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	var err error
	vpc := a.Vpc

	if vpc.SecurityGroupId != "" {
		log.Infof("Delete security group: %s", vpc.SecurityGroupId)
		if err = vpc.deleteSecurityGroup(); err != nil {
			return err
		}
	}

	if vpc.SubnetId != "" {
		log.Infof("Delete subnet: %s", vpc.SubnetId)
		if err = vpc.deleteSubnet(); err != nil {
			return err
		}
	}

	if vpc.ID != "" {
		log.Infof("Delete vpc: %s", vpc.ID)
		if err = vpc.deleteVpc(); err != nil {
			return err
		}
	}

	return nil
}

func (a *AWSProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	credentials, _ := a.AwsConfig.Credentials.Retrieve(context.TODO())

	return map[string]string{
		"pause_image":          a.PauseImage,
		"podvm_launchtemplate": "",
		"podvm_ami":            a.Image.ID,
		"podvm_instance_type":  "t3.small",
		"sg_ids":               a.Vpc.SecurityGroupId, // TODO: what other SG needed?
		"subnet_id":            a.Vpc.SubnetId,
		"ssh_kp_name":          a.SshKpName,
		"region":               a.AwsConfig.Region,
		"access_key_id":        credentials.AccessKeyID,
		"secret_access_key":    credentials.SecretAccessKey,
		"vxlan_port":           a.VxlanPort,
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
	log.Infof("Create vmimport service role")
	if err = createVmimportServiceRole(ctx, a.iamClient, a.Bucket.Name); err != nil {
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

	imageName := strings.Replace(filepath.Base(imagePath), ".qcow2", ".raw", 1)
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
		cidrBlock = "10.0.0.0/16"
	}

	return &Vpc{
		BaseName:          "caa-e2e-test",
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
		CidrBlock: aws.String(v.CidrBlock),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeVpc,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(v.BaseName + "-vpc"),
					},
				},
			},
		},
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
			VpcId:     aws.String(v.ID),
			CidrBlock: aws.String("10.0.0.0/24"),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeSubnet,
					Tags: []ec2types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(v.BaseName + "-subnet"),
						},
					},
				},
			},
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
			AvailabilityZone: aws.String(secondarySubnetAz),
			VpcId:            aws.String(v.ID),
			CidrBlock:        aws.String("10.0.0.0/24"),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeSubnet,
					Tags: []ec2types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(v.BaseName + "-subnet-2"),
						},
					},
				},
			},
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
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeInternetGateway,
					Tags: []ec2types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(v.BaseName + "-igw"),
						},
					},
				},
			},
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
			VpcId: aws.String(v.ID),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeRouteTable,
					Tags: []ec2types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(v.BaseName + "-rtb"),
						},
					},
				},
			},
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
			Description: aws.String("cloud-api-adaptor e2e tests"),
			GroupName:   aws.String(v.BaseName + "-sg"),
			VpcId:       aws.String(v.ID),
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

// createBucket Creates the S3 bucket
func (b *S3Bucket) createBucket() error {
	// No harm creating a bucket that already exist.
	_, err := b.Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: &b.Name,
	})
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

// createVmimportServiceRole Creates the vmimport service role as required to use the VM snaphot import feature.
//
//	For further details see https://docs.aws.amazon.com/vm-import/latest/userguide/required-permissions.html
func createVmimportServiceRole(ctx context.Context, client *iam.Client, bucketName string) error {
	const roleName = "vmimport"

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
	return &AMIImage{
		Client:          client,
		Description:     "Peer Pod VM image",
		DiskDescription: "Peer Pod VM disk",
		DiskFormat:      "RAW",
		EBSSnapshotId:   "", // To be defined when the snapshot is created
		ID:              properties["podvm_aws_ami_id"],
		RootDeviceName:  "/dev/xvda",
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
	if err = waiter.Wait(context.TODO(), describeTasksInput, time.Minute*3); err != nil {
		return err
	}

	// Finally get the snapshot ID
	describeTasks, err := i.Client.DescribeImportSnapshotTasks(context.TODO(), describeTasksInput)
	if err != nil {
		return err
	}
	taskDetail := describeTasks.ImportSnapshotTasks[0].SnapshotTaskDetail
	i.EBSSnapshotId = *taskDetail.SnapshotId

	return nil
}

// registerImage Registers an AMI image
func (i *AMIImage) registerImage(imageName string) error {

	if i.EBSSnapshotId == "" {
		return fmt.Errorf("EBS Snapshot ID not found\n")
	}

	result, err := i.Client.RegisterImage(context.TODO(), &ec2.RegisterImageInput{
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
	})
	if err != nil {
		return err
	}

	// Save the AMI ID
	i.ID = *result.ImageId
	return nil
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
	cmd := exec.Command("aws", "s3", "cp", filepath, s3uri)
	out, err := cmd.CombinedOutput()
	fmt.Printf("%s\n", out)
	if err != nil {
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

func NewAwsInstallOverlay(installDir string) (InstallOverlay, error) {
	overlay, err := NewKustomizeOverlay(filepath.Join(installDir, "overlays/aws"))
	if err != nil {
		return nil, err
	}

	return &AwsInstallOverlay{
		overlay: overlay,
	}, nil
}

func (a *AwsInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return a.overlay.Apply(ctx, cfg)
}

func (a *AwsInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return a.overlay.Delete(ctx, cfg)
}

func (a *AwsInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	var err error

	// Mapping the internal properties to ConfigMapGenerator properties.
	mapProps := map[string]string{
		"pause_image":          "PAUSE_IMAGE",
		"podvm_launchtemplate": "PODVM_LAUNCHTEMPLATE_NAME",
		"podvm_ami":            "PODVM_AMI_ID",
		"podvm_instance_type":  "PODVM_INSTANCE_TYPE",
		"sg_ids":               "AWS_SG_IDS",
		"subnet_id":            "AWS_SUBNET_ID",
		"ssh_kp_name":          "SSH_KP_NAME",
		"region":               "AWS_REGION",
		"vxlan_port":           "VXLAN_PORT",
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			if err = a.overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm",
				v, properties[k]); err != nil {
				return err
			}
		}
	}

	// Mapping the internal properties to SecretGenerator properties.
	mapProps = map[string]string{
		"access_key_id":     "AWS_ACCESS_KEY_ID",
		"secret_access_key": "AWS_SECRET_ACCESS_KEY",
	}
	for k, v := range mapProps {
		if properties[k] != "" {
			if err = a.overlay.SetKustomizeSecretGeneratorLiteral("peer-pods-secret",
				v, properties[k]); err != nil {
				return err
			}
		}
	}

	if err = a.overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}
