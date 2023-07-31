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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/aws] ", log.LstdFlags|log.Lmsgprefix)
var errNotReady = errors.New("address not ready")

const maxInstanceNameLen = 63

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
}
type awsProvider struct {
	// Make ec2Client a mockable interface
	ec2Client     ec2Client
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

	provider := &awsProvider{
		ec2Client:     ec2Client,
		serviceConfig: config,
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
