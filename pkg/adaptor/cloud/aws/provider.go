// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/aws] ", log.LstdFlags|log.Lmsgprefix)
var errNotReady = errors.New("address not ready")

const (
	maxInstanceNameLen = 63
	maxWaitTime        = 120 * time.Second
)

// Make ec2Client a mockable interface
type ec2Client interface {
	RunInstances(ctx context.Context,
		params *ec2.RunInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	TerminateInstances(ctx context.Context,
		params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	// Add DescribeInstanceTypes method
	DescribeInstanceTypes(ctx context.Context,
		params *ec2.DescribeInstanceTypesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
	// Add DescribeInstances method
	DescribeInstances(ctx context.Context,
		params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	// Add DescribeImages method
	DescribeImages(ctx context.Context,
		params *ec2.DescribeImagesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
}

// Make instanceRunningWaiter as an interface
type instanceRunningWaiter interface {
	Wait(ctx context.Context,
		params *ec2.DescribeInstancesInput,
		maxWaitDur time.Duration,
		optFns ...func(*ec2.InstanceRunningWaiterOptions)) error
}

type awsProvider struct {
	// Make ec2Client a mockable interface
	ec2Client ec2Client
	// Make waiter a mockable interface
	waiter        instanceRunningWaiter
	serviceConfig *Config
}

func NewProvider(config *Config) (cloud.Provider, error) {

	logger.Printf("aws config: %#v", config.Redact())

	if err := retrieveMissingConfig(config); err != nil {
		logger.Printf("Failed to retrieve configuration, some fields may still be missing: %v", err)
	}

	ec2Client, err := NewEC2Client(*config)
	if err != nil {
		return nil, err
	}

	waiter := ec2.NewInstanceRunningWaiter(ec2Client)

	provider := &awsProvider{
		ec2Client:     ec2Client,
		waiter:        waiter,
		serviceConfig: config,
	}

	// If root volume size is set, then get the device name from the AMI and update the serviceConfig
	if config.RootVolumeSize > 0 {
		// Get the device name from the AMI
		deviceName, deviceSize, err := provider.getDeviceNameAndSize(config.ImageId)
		if err != nil {
			return nil, err
		}

		// If RootVolumeSize < deviceSize, then update the RootVolumeSize to deviceSize
		if config.RootVolumeSize < int(deviceSize) {
			logger.Printf("RootVolumeSize %d is less than deviceSize %d, hence updating RootVolumeSize to deviceSize",
				config.RootVolumeSize, deviceSize)
			config.RootVolumeSize = int(deviceSize)
		}

		// Update the serviceConfig with the device name
		config.RootDeviceName = deviceName

		logger.Printf("RootDeviceName and RootVolumeSize of the image %s is %s, %d", config.ImageId, config.RootDeviceName, config.RootVolumeSize)
	}

	if err = provider.updateInstanceTypeSpecList(); err != nil {
		return nil, err
	}

	return provider, nil
}

func getIPs(instance types.Instance) ([]netip.Addr, error) {

	var podNodeIPs []netip.Addr
	for i, nic := range instance.NetworkInterfaces {
		addr := nic.PrivateIpAddress

		if addr == nil || *addr == "" || *addr == "0.0.0.0" {
			return nil, errNotReady
		}

		ip, err := netip.ParseAddr(*addr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod node IP %q: %w", *addr, err)
		}
		podNodeIPs = append(podNodeIPs, ip)

		logger.Printf("podNodeIP[%d]=%s", i, ip.String())
	}

	return podNodeIPs, nil
}

func (p *awsProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec cloud.InstanceTypeSpec) (*cloud.Instance, error) {

	// Public IP address
	var publicIPAddr netip.Addr

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))

	instanceType, err := p.selectInstanceType(ctx, spec)
	if err != nil {
		return nil, err
	}

	instanceTags := []types.Tag{
		{
			Key:   aws.String("Name"),
			Value: aws.String(instanceName),
		},
	}

	// Add custom tags (k=v) from serviceConfig.Tags to the instance
	for k, v := range p.serviceConfig.Tags {
		instanceTags = append(instanceTags, types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	// Create TagSpecifications for the instance
	tagSpecifications := []types.TagSpecification{
		{
			ResourceType: types.ResourceTypeInstance,
			Tags:         instanceTags,
		},
	}

	var input *ec2.RunInstancesInput

	if p.serviceConfig.UseLaunchTemplate {
		input = &ec2.RunInstancesInput{
			MinCount: aws.Int32(1),
			MaxCount: aws.Int32(1),
			LaunchTemplate: &types.LaunchTemplateSpecification{
				LaunchTemplateName: aws.String(p.serviceConfig.LaunchTemplateName),
			},
			UserData:          &userDataEnc,
			TagSpecifications: tagSpecifications,
		}
	} else {
		input = &ec2.RunInstancesInput{
			MinCount:          aws.Int32(1),
			MaxCount:          aws.Int32(1),
			ImageId:           aws.String(p.serviceConfig.ImageId),
			InstanceType:      types.InstanceType(instanceType),
			SecurityGroupIds:  p.serviceConfig.SecurityGroupIds,
			SubnetId:          aws.String(p.serviceConfig.SubnetId),
			UserData:          &userDataEnc,
			TagSpecifications: tagSpecifications,
		}
		if p.serviceConfig.KeyName != "" {
			input.KeyName = aws.String(p.serviceConfig.KeyName)
		}

		// Auto assign public IP address if UsePublicIP is set
		if p.serviceConfig.UsePublicIP {
			// Auto-assign public IP
			input.NetworkInterfaces = []types.InstanceNetworkInterfaceSpecification{
				{
					AssociatePublicIpAddress: aws.Bool(true),
					DeviceIndex:              aws.Int32(0),
					SubnetId:                 aws.String(p.serviceConfig.SubnetId),
					Groups:                   p.serviceConfig.SecurityGroupIds,
					DeleteOnTermination:      aws.Bool(true),
				},
			}
			// Remove the subnet ID from the input
			input.SubnetId = nil
			// Remove the security group IDs from the input
			input.SecurityGroupIds = nil

		}
	}

	// Add block device mappings to the instance to set the root volume size
	if p.serviceConfig.RootVolumeSize > 0 {
		input.BlockDeviceMappings = []types.BlockDeviceMapping{
			{
				DeviceName: aws.String(p.serviceConfig.RootDeviceName),
				Ebs: &types.EbsBlockDevice{
					VolumeSize: aws.Int32(int32(p.serviceConfig.RootVolumeSize)),
				},
			},
		}
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	result, err := p.ec2Client.RunInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("Creating instance (%v) returned error: %s", result, err)
	}

	logger.Printf("created an instance %s for sandbox %s", *result.Instances[0].PublicDnsName, sandboxID)

	instanceID := *result.Instances[0].InstanceId

	ips, err := getIPs(result.Instances[0])
	if err != nil {
		logger.Printf("failed to get IPs for the instance : %v ", err)
		return nil, err
	}

	if p.serviceConfig.UsePublicIP {
		// Get the public IP address of the instance
		publicIPAddr, err = p.getPublicIP(ctx, instanceID)
		if err != nil {

			return nil, err
		}

		// Replace the first IP address with the public IP address
		ips[0] = publicIPAddr

	}

	instance := &cloud.Instance{
		ID:   instanceID,
		Name: instanceName,
		IPs:  ips,
	}

	return instance, nil
}

func (p *awsProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	terminateInput := &ec2.TerminateInstancesInput{
		InstanceIds: []string{
			instanceID,
		},
	}

	logger.Printf("Deleting instance (%s)", instanceID)

	resp, err := p.ec2Client.TerminateInstances(ctx, terminateInput)

	if err != nil {
		logger.Printf("failed to delete an instance: %v and the response is %v", err, resp)
		return err
	}
	logger.Printf("deleted an instance %s", instanceID)
	return nil

}

func (p *awsProvider) Teardown() error {
	return nil
}

// Add SelectInstanceType method to select an instance type based on the memory and vcpu requirements
func (p *awsProvider) selectInstanceType(ctx context.Context, spec cloud.InstanceTypeSpec) (string, error) {

	return cloud.SelectInstanceTypeToUse(spec, p.serviceConfig.InstanceTypeSpecList, p.serviceConfig.InstanceTypes, p.serviceConfig.InstanceType)
}

// Add a method to populate InstanceTypeSpecList for all the instanceTypes
func (p *awsProvider) updateInstanceTypeSpecList() error {

	// Get the instance types from the service config
	instanceTypes := p.serviceConfig.InstanceTypes

	// If instanceTypes is empty then populate it with the default instance type
	if len(instanceTypes) == 0 {
		instanceTypes = append(instanceTypes, p.serviceConfig.InstanceType)
	}

	// Create a list of instancetypespec
	var instanceTypeSpecList []cloud.InstanceTypeSpec

	// Iterate over the instance types and populate the instanceTypeSpecList
	for _, instanceType := range instanceTypes {
		vcpus, memory, err := p.getInstanceTypeInformation(instanceType)
		if err != nil {
			return err
		}
		instanceTypeSpecList = append(instanceTypeSpecList, cloud.InstanceTypeSpec{InstanceType: instanceType, VCPUs: vcpus, Memory: memory})
	}

	// Sort the instanceTypeSpecList by Memory and update the serviceConfig
	p.serviceConfig.InstanceTypeSpecList = cloud.SortInstanceTypesOnMemory(instanceTypeSpecList)
	logger.Printf("InstanceTypeSpecList (%v)", p.serviceConfig.InstanceTypeSpecList)
	return nil
}

// Add a method to retrieve cpu, memory, and storage from the instance type
func (p *awsProvider) getInstanceTypeInformation(instanceType string) (vcpu int64, memory int64, err error) {

	// Get the instance type information from the instance type using AWS API
	input := &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []types.InstanceType{
			types.InstanceType(instanceType),
		},
	}
	// Get the instance type information from the instance type using AWS API
	result, err := p.ec2Client.DescribeInstanceTypes(context.Background(), input)
	if err != nil {
		return 0, 0, err
	}

	// Get the vcpu and memory from the result
	if len(result.InstanceTypes) > 0 {
		vcpu = int64(*result.InstanceTypes[0].VCpuInfo.DefaultVCpus)
		memory = int64(*result.InstanceTypes[0].MemoryInfo.SizeInMiB)
		return vcpu, memory, nil
	}
	return 0, 0, fmt.Errorf("instance type %s not found", instanceType)

}

// Add a method to get public IP address of the instance
// Take the instance id as an argument
// Return the public IP address as a string
func (p *awsProvider) getPublicIP(ctx context.Context, instanceID string) (netip.Addr, error) {
	// Add describe instance input
	describeInstanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	// Create New InstanceRunningWaiter
	//waiter := ec2.NewInstanceRunningWaiter(p.ec2Client)

	// Wait for instance to be ready before getting the public IP address
	err := p.waiter.Wait(ctx, describeInstanceInput, maxWaitTime)
	if err != nil {
		logger.Printf("failed to wait for the instance to be ready : %v ", err)
		return netip.Addr{}, err

	}

	// Add describe instance output
	describeInstanceOutput, err := p.ec2Client.DescribeInstances(ctx, describeInstanceInput)
	if err != nil {
		logger.Printf("failed to describe the instance : %v ", err)
		return netip.Addr{}, err
	}
	// Get the public IP address from InstanceNetworkInterfaceAssociation
	publicIP := describeInstanceOutput.Reservations[0].Instances[0].NetworkInterfaces[0].Association.PublicIp

	// Check if the public IP address is nil
	if publicIP == nil {
		return netip.Addr{}, fmt.Errorf("public IP address is nil")
	}
	// If the public IP address is empty, return an error
	if *publicIP == "" {
		return netip.Addr{}, fmt.Errorf("public IP address is empty")
	}

	logger.Printf("public IP address of the instance %s is %s", instanceID, *publicIP)

	// Parse the public IP address
	publicIPAddr, err := netip.ParseAddr(*publicIP)
	if err != nil {
		return netip.Addr{}, err
	}

	return publicIPAddr, nil
}

func (p *awsProvider) getDeviceNameAndSize(imageID string) (string, int32, error) {
	// Add describe images input
	describeImagesInput := &ec2.DescribeImagesInput{
		ImageIds: []string{imageID},
	}

	// Add describe images output
	describeImagesOutput, err := p.ec2Client.DescribeImages(context.Background(), describeImagesInput)
	if err != nil {
		logger.Printf("failed to describe the image : %v ", err)
		return "", 0, err
	}

	// Get the device name
	deviceName := describeImagesOutput.Images[0].RootDeviceName

	// Check if the device name is nil
	if deviceName == nil {
		return "", 0, fmt.Errorf("device name is nil")
	}
	// If the device name is empty, return an error
	if *deviceName == "" {
		return "", 0, fmt.Errorf("device name is empty")
	}

	// Get the device size if it is set
	deviceSize := describeImagesOutput.Images[0].BlockDeviceMappings[0].Ebs.VolumeSize

	if deviceSize == nil {
		logger.Printf("device size of the image %s is not set", imageID)
		return *deviceName, 0, nil
	}

	logger.Printf("device name and size of the image %s is %s, %d", imageID, *deviceName, *deviceSize)

	return *deviceName, *deviceSize, nil
}
