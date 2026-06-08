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
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"golang.org/x/sync/errgroup"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

var (
	logger = log.New(log.Writer(), "[adaptor/cloud/aws] ", log.LstdFlags|log.Lmsgprefix)

	errNotReady             = errors.New("address not ready")
	errNoImageID            = errors.New("ImageId is empty")
	errNilPublicIPAddress   = errors.New("public IP address is nil")
	errEmptyPublicIPAddress = errors.New("public IP address is empty")
	errImageDetailsFailed   = errors.New("unable to get image details")
	errDeviceNameEmpty      = errors.New("empty device name")
)

const (
	maxInstanceNameLen = 63
	maxWaitTime        = 120 * time.Second
	maxInt32           = 1<<31 - 1
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
	// Add AllocateAddress method
	AllocateAddress(ctx context.Context,
		params *ec2.AllocateAddressInput,
		optFns ...func(*ec2.Options)) (*ec2.AllocateAddressOutput, error)
	AssociateAddress(ctx context.Context,
		params *ec2.AssociateAddressInput,
		optFns ...func(*ec2.Options)) (*ec2.AssociateAddressOutput, error)
	DescribeAddresses(ctx context.Context,
		params *ec2.DescribeAddressesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	ReleaseAddress(ctx context.Context,
		params *ec2.ReleaseAddressInput,
		optFns ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error)
	DisassociateAddress(ctx context.Context,
		params *ec2.DisassociateAddressInput,
		optFns ...func(*ec2.Options)) (*ec2.DisassociateAddressOutput, error)
	CreateNetworkInterface(ctx context.Context,
		params *ec2.CreateNetworkInterfaceInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateNetworkInterfaceOutput, error)
	AttachNetworkInterface(ctx context.Context,
		params *ec2.AttachNetworkInterfaceInput,
		optFns ...func(*ec2.Options)) (*ec2.AttachNetworkInterfaceOutput, error)
	DeleteNetworkInterface(ctx context.Context,
		params *ec2.DeleteNetworkInterfaceInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInterfaceOutput, error)
	ModifyNetworkInterfaceAttribute(ctx context.Context,
		params *ec2.ModifyNetworkInterfaceAttributeInput,
		optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput, error)
	AttachVolume(ctx context.Context,
		params *ec2.AttachVolumeInput,
		optFns ...func(*ec2.Options)) (*ec2.AttachVolumeOutput, error)
	DescribeVolumes(ctx context.Context,
		params *ec2.DescribeVolumesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
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

func NewProvider(config *Config) (provider.Provider, error) {
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
		deviceName, deviceSize, err := provider.getDeviceNameAndSize(config.ImageID)
		if err != nil {
			return nil, err
		}

		// If RootVolumeSize < deviceSize, then update the RootVolumeSize to deviceSize
		if config.RootVolumeSize < int(deviceSize) {
			logger.Printf("RootVolumeSize %d is less than deviceSize %d, hence updating RootVolumeSize to deviceSize",
				config.RootVolumeSize, deviceSize)
			config.RootVolumeSize = int(deviceSize)
		}

		// Ensure RootVolumeSize is not more than max int32
		// The AWS apis accepts only int32, however the flags package has only IntVar.
		// So we can't make RootVolumeSize as int32, hence checking for overflow here.

		if config.RootVolumeSize > maxInt32 {
			logger.Printf("RootVolumeSize %d exceeds max int32 value, setting to max int32", config.RootVolumeSize)
			config.RootVolumeSize = maxInt32
		}

		// Update the serviceConfig with the device name
		config.RootDeviceName = deviceName

		logger.Printf("RootDeviceName and RootVolumeSize of the image %s is %s, %d", config.ImageID, config.RootDeviceName, config.RootVolumeSize)
	}

	if err := provider.updateInstanceTypeSpecList(); err != nil {
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

		logger.Printf("instance %s: podNodeIP[%d]=%s", *instance.InstanceId, i, ip.String())
	}

	return podNodeIPs, nil
}

func (p *awsProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (instance *provider.Instance, err error) {
	// Public IP address
	var publicIPAddr netip.Addr

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	// Convert userData to base64
	b64EncData := base64.StdEncoding.EncodeToString([]byte(cloudConfigData))

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
			UserData:          &b64EncData,
			TagSpecifications: tagSpecifications,
		}
	} else {

		imageID := p.serviceConfig.ImageID

		if spec.Image != "" {
			logger.Printf("Choosing %s from annotation as the AWS AMI for the PodVM image", spec.Image)
			imageID = spec.Image
		}

		input = &ec2.RunInstancesInput{
			MinCount:          aws.Int32(1),
			MaxCount:          aws.Int32(1),
			ImageId:           aws.String(imageID),
			InstanceType:      types.InstanceType(instanceType),
			SecurityGroupIds:  p.serviceConfig.SecurityGroupIDs,
			SubnetId:          aws.String(p.serviceConfig.SubnetID),
			UserData:          &b64EncData,
			TagSpecifications: tagSpecifications,
		}
		if p.serviceConfig.KeyName != "" {
			input.KeyName = aws.String(p.serviceConfig.KeyName)
		}

		if p.serviceConfig.UsePublicIP {
			// Auto-assign public IP
			input.NetworkInterfaces = []types.InstanceNetworkInterfaceSpecification{
				{
					AssociatePublicIpAddress: aws.Bool(true),
					DeviceIndex:              aws.Int32(0),
					SubnetId:                 aws.String(p.serviceConfig.SubnetID),
					Groups:                   p.serviceConfig.SecurityGroupIDs,
					DeleteOnTermination:      aws.Bool(true),
				},
			}
			// Remove the subnet ID from the input
			input.SubnetId = nil
			// Remove the security group IDs from the input
			input.SecurityGroupIds = nil
		}

		// Ref: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/snp-work.html
		// Use the following CLI command to retrieve the list of instance types that support AMD SEV-SNP:
		// aws ec2 describe-instance-types \
		//--filters Name=processor-info.supported-features,Values=amd-sev-snp \
		//--query 'InstanceTypes[*].InstanceType'
		// Using AMD SEV-SNP requires an AMI with uefi or uefi-preferred boot enabled
		if !p.serviceConfig.DisableCVM {
			//  Add AmdSevSnp Cpu options to the instance
			input.CpuOptions = &types.CpuOptionsRequest{
				// Add AmdSevSnp Cpu options to the instance
				AmdSevSnp: types.AmdSevSnpSpecificationEnabled,
			}
		}
	}

	// Add block device mappings to the instance to set the root volume size
	if p.serviceConfig.RootVolumeSize > 0 {
		input.BlockDeviceMappings = []types.BlockDeviceMapping{
			{
				DeviceName: aws.String(p.serviceConfig.RootDeviceName),
				Ebs: &types.EbsBlockDevice{
					// We have already ensured RootVolumeSize is not more than max int32 in NewProvider
					// Hence we can safely convert it to int32
					VolumeSize: aws.Int32(int32(p.serviceConfig.RootVolumeSize)),
				},
			},
		}
	}

	if len(spec.Volumes) > 0 {
		if err := p.validateVolumes(ctx, spec.Volumes); err != nil {
			return nil, err
		}
	}

	logger.Printf("Creating instance %s for sandbox %s", instanceName, sandboxID)

	result, err := p.ec2Client.RunInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("creating instance %s (%v): %w", instanceName, result, err)
	}

	instanceID := *result.Instances[0].InstanceId

	logger.Printf("Created instance %s (%s) for sandbox %s", instanceName, instanceID, sandboxID)

	// Create partial instance to return on error (allows caller to cleanup)
	instance = &provider.Instance{
		ID:   instanceID,
		Name: instanceName,
	}

	ips, err := getIPs(result.Instances[0])
	if err != nil {
		logger.Printf("Failed to get IPs for instance %s: %v ", instanceID, err)
		return instance, err
	}

	if p.serviceConfig.UsePublicIP {
		// Get the public IP address of the instance
		publicIPAddr, err = p.getPublicIP(ctx, instanceID)
		if err != nil {
			return instance, err
		}

		// Replace the first IP address with the public IP address
		ips[0] = publicIPAddr
	}

	if len(spec.Volumes) > 0 {
		if err := p.attachEBSVolumes(ctx, instanceID, spec.Volumes); err != nil {
			return instance, fmt.Errorf("attaching EBS volumes to %s: %w", instanceID, err)
		}
	}

	if spec.MultiNic {
		nIfaceID, err := p.createAddonNICforInstance(ctx, instanceID)
		if err != nil {
			return instance, err
		}
		// If public IP is set, then create an ElasticIP and associate it with this secondary interface
		if p.serviceConfig.UsePublicIP {
			err = p.createElasticIPforInstance(ctx, instanceID, nIfaceID)
			if err != nil {
				return instance, err
			}
		}
	}

	instance.IPs = ips

	return instance, nil
}

const (
	ebsAttachTimeout  = 60 * time.Second
	ebsAttachPollRate = 3 * time.Second
)

// ebsDeviceName generates /dev/xvdb../dev/xvdz for indices 0-24, then
// /dev/xvdba../dev/xvdbz for 25-50, etc. This avoids the 25-disk
// limit of the previous single-letter scheme.
func ebsDeviceName(index int) string {
	if index < 25 {
		return fmt.Sprintf("/dev/xvd%c", 'b'+rune(index))
	}
	first := 'b' + rune((index-25)/26)
	second := 'a' + rune((index-25)%26)
	return fmt.Sprintf("/dev/xvd%c%c", first, second)
}

var ebsVolumeIDRe = regexp.MustCompile(`^vol-[0-9a-f]{8,}$`)

func isValidEBSVolumeID(id string) bool {
	return ebsVolumeIDRe.MatchString(id)
}

// validateVolumes checks that all EBS volumes exist and are in the "available"
// state before creating the PodVM instance.
func (p *awsProvider) validateVolumes(ctx context.Context, volumes []provider.CloudVolume) error {
	var volumeIDs []string
	for _, vol := range volumes {
		if !isValidEBSVolumeID(vol.DiskID) {
			return fmt.Errorf("invalid EBS volume ID format: %q", vol.DiskID)
		}
		volumeIDs = append(volumeIDs, vol.DiskID)
	}

	result, err := p.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		VolumeIds: volumeIDs,
	})
	if err != nil {
		return fmt.Errorf("validating EBS volumes: %w", err)
	}

	var errs []string
	for _, v := range result.Volumes {
		if v.State != types.VolumeStateAvailable {
			attached := ""
			for _, att := range v.Attachments {
				if att.InstanceId != nil {
					attached = fmt.Sprintf(" (attached to %s)", *att.InstanceId)
				}
			}
			errs = append(errs, fmt.Sprintf("volume %s is in state %q%s",
				*v.VolumeId, v.State, attached))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("EBS volume pre-validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// attachEBSVolumes attaches the given EBS volumes to the instance.
// IMPORTANT: EBS volumes can only be attached to instances in the same
// Availability Zone. Ensure the PodVM instance and volumes are in the same AZ
// (e.g. via subnet placement or StorageClass allowedTopologies).
func (p *awsProvider) attachEBSVolumes(ctx context.Context, instanceID string, volumes []provider.CloudVolume) error {
	if len(volumes) == 0 {
		return nil
	}

	describeInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}
	if err := p.waiter.Wait(ctx, describeInput, maxWaitTime); err != nil {
		return fmt.Errorf("waiting for instance %s to be running: %w", instanceID, err)
	}

	g, gctx := errgroup.WithContext(ctx)

	for i, vol := range volumes {
		idx := i
		diskID := vol.DiskID
		deviceName := ebsDeviceName(idx)

		g.Go(func() error {
			logger.Printf("Attaching EBS volume %s to instance %s as %s (index %d)", diskID, instanceID, deviceName, idx)

			attachOutput, err := p.ec2Client.AttachVolume(gctx, &ec2.AttachVolumeInput{
				Device:     aws.String(deviceName),
				InstanceId: aws.String(instanceID),
				VolumeId:   aws.String(diskID),
			})
			if err != nil {
				return fmt.Errorf("attaching EBS volume %s: %w", diskID, err)
			}

			logger.Printf("AttachVolume request accepted for %s (state: %s)", diskID, attachOutput.State)

			if err := p.waitForVolumeAttached(gctx, diskID); err != nil {
				return fmt.Errorf("waiting for EBS volume %s to attach: %w", diskID, err)
			}

			logger.Printf("EBS volume %s attached successfully to %s as %s", diskID, instanceID, deviceName)
			return nil
		})
	}

	return g.Wait()
}

func (p *awsProvider) waitForVolumeAttached(ctx context.Context, volumeID string) error {
	deadline := time.Now().Add(ebsAttachTimeout)
	for time.Now().Before(deadline) {
		result, err := p.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			VolumeIds: []string{volumeID},
		})
		if err != nil {
			return fmt.Errorf("describing volume %s: %w", volumeID, err)
		}
		if len(result.Volumes) > 0 && len(result.Volumes[0].Attachments) > 0 {
			state := result.Volumes[0].Attachments[0].State
			if state == types.VolumeAttachmentStateAttached {
				return nil
			}
			logger.Printf("Volume %s attachment state: %s, waiting...", volumeID, state)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(ebsAttachPollRate):
		}
	}
	return fmt.Errorf("volume %s did not reach attached state within %v", volumeID, ebsAttachTimeout)
}

// DeleteInstance terminates the EC2 instance. EBS volumes attached after
// launch (with DeleteOnTermination=false, the default) are automatically
// detached by AWS when the instance terminates.
func (p *awsProvider) DeleteInstance(ctx context.Context, instanceID string) error {

	err := p.deleteElasticIPforInstance(ctx, instanceID)
	if err != nil {
		logger.Printf("failed to deallocate the Elastic IP address: %v", err)
	}

	terminateInput := &ec2.TerminateInstancesInput{
		InstanceIds: []string{
			instanceID,
		},
	}

	logger.Printf("Deleting instance %s", instanceID)

	resp, err := p.ec2Client.TerminateInstances(ctx, terminateInput)
	if err != nil {
		logger.Printf("failed to delete instance %v: %v and the response is %v", instanceID, err, resp)
		return err
	}

	logger.Printf("Deleted instance %s", instanceID)

	return nil
}

func (p *awsProvider) Teardown() error {
	return nil
}

func (p *awsProvider) ConfigVerifier() error {
	if len(p.serviceConfig.ImageID) == 0 {
		return errNoImageID
	}
	return nil
}

// Add SelectInstanceType method to select an instance type based on the memory and vcpu requirements
func (p *awsProvider) selectInstanceType(_ context.Context, spec provider.InstanceTypeSpec) (string, error) {
	return provider.SelectInstanceTypeToUse(spec, p.serviceConfig.InstanceTypeSpecList, p.serviceConfig.InstanceTypes, p.serviceConfig.InstanceType)
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
	var instanceTypeSpecList []provider.InstanceTypeSpec

	// Iterate over the instance types and populate the instanceTypeSpecList
	for _, instanceType := range instanceTypes {
		vcpus, memory, gpuCount, err := p.getInstanceTypeInformation(instanceType)
		if err != nil {
			return err
		}
		instanceTypeSpecList = append(instanceTypeSpecList,
			provider.InstanceTypeSpec{InstanceType: instanceType, VCPUs: vcpus, Memory: memory, GPUs: gpuCount})
	}

	// Sort the instanceTypeSpecList and update the serviceConfig
	p.serviceConfig.InstanceTypeSpecList = provider.SortInstanceTypesOnResources(instanceTypeSpecList)
	logger.Printf("InstanceTypeSpecList (%v)", p.serviceConfig.InstanceTypeSpecList)
	return nil
}

var errInstanceTypeNotFound = errors.New("instance type not found")

// Add a method to retrieve cpu, memory, and storage from the instance type
func (p *awsProvider) getInstanceTypeInformation(instanceType string) (int64, int64,
	int64, error,
) {
	// Get the instance type information from the instance type using AWS API
	input := &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []types.InstanceType{
			types.InstanceType(instanceType),
		},
	}
	// Get the instance type information from the instance type using AWS API
	result, err := p.ec2Client.DescribeInstanceTypes(context.Background(), input)
	if err != nil {
		return 0, 0, 0, err
	}

	// Get the vcpu, memory and gpu from the result
	if len(result.InstanceTypes) > 0 {
		instanceInfo := result.InstanceTypes[0]
		vcpu := int64(*instanceInfo.VCpuInfo.DefaultVCpus)
		memory := *instanceInfo.MemoryInfo.SizeInMiB

		// Get the GPU information
		gpuCount := int64(0)
		if instanceInfo.GpuInfo != nil {
			for _, gpu := range instanceInfo.GpuInfo.Gpus {
				gpuCount += int64(*gpu.Count)
			}
		}

		return vcpu, memory, gpuCount, nil
	}

	return 0, 0, 0, errInstanceTypeNotFound
}

// Add a method to get public IP address of the instance
// Take the instance id as an argument
// Return the public IP address as a string
func (p *awsProvider) getPublicIP(ctx context.Context, instanceID string) (netip.Addr, error) {
	// Add describe instance input
	describeInstanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	// Wait for instance to be ready before getting the public IP address
	if err := p.waiter.Wait(ctx, describeInstanceInput, maxWaitTime); err != nil {
		logger.Printf("failed to wait for instance %s to be ready: %v", instanceID, err)
		return netip.Addr{}, err
	}

	// Add describe instance output
	describeInstanceOutput, err := p.ec2Client.DescribeInstances(ctx, describeInstanceInput)
	if err != nil {
		logger.Printf("failed to describe instance %s: %v", instanceID, err)
		return netip.Addr{}, err
	}
	// Get the public IP address from InstanceNetworkInterfaceAssociation
	publicIP := describeInstanceOutput.Reservations[0].Instances[0].NetworkInterfaces[0].Association.PublicIp

	// Check if the public IP address is nil
	if publicIP == nil {
		return netip.Addr{}, errNilPublicIPAddress
	}
	// If the public IP address is empty, return an error
	if *publicIP == "" {
		return netip.Addr{}, errEmptyPublicIPAddress
	}

	logger.Printf("public IP address instance %s: %s", instanceID, *publicIP)

	// Parse the public IP address
	return netip.ParseAddr(*publicIP)
}

// Create a NIC and attach it to the instance
func (p *awsProvider) createAddonNICforInstance(ctx context.Context, instanceID string) (nIfaceID *string, err error) {
	// Create network interface
	// Add create network interface input
	nicName := fmt.Sprintf("nic-%s", instanceID)
	createNetworkInterfaceInput := &ec2.CreateNetworkInterfaceInput{
		SubnetId: aws.String(p.serviceConfig.SubnetID),
		Groups:   p.serviceConfig.SecurityGroupIDs,

		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeNetworkInterface,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(nicName),
					},
				},
			},
		},
	}

	nic, err := p.ec2Client.CreateNetworkInterface(ctx, createNetworkInterfaceInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create a network interface: %v", err)
	}

	nIfaceID = nic.NetworkInterface.NetworkInterfaceId

	// Wait for instance to be ready before attaching the network interface
	describeInstanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}
	err = p.waiter.Wait(ctx, describeInstanceInput, maxWaitTime)
	if err != nil {
		logger.Printf("failed to wait for the instance to be ready : %v ", err)
		return nil, err

	}

	// Attach network interface
	attachNetworkInterfaceInput := &ec2.AttachNetworkInterfaceInput{
		InstanceId:         aws.String(instanceID),
		NetworkInterfaceId: nIfaceID,
		DeviceIndex:        aws.Int32(1),
	}

	nicAttachOp, err := p.ec2Client.AttachNetworkInterface(ctx, attachNetworkInterfaceInput)
	if err != nil {
		_, nicDelErr := p.ec2Client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: nIfaceID,
		})
		if nicDelErr != nil {
			logger.Printf("failed to delete the network interface: %v", nicDelErr)
		}

		return nil, fmt.Errorf("failed to attach a network interface: %v", err)
	}

	if nicAttachOp.AttachmentId == nil {
		logger.Printf("AttachmentId is nil. This will prevent deletion of the network interface")
		return nil, fmt.Errorf("AttachmentId is nil")
	}

	// Set Delete on termination to true
	_, err = p.ec2Client.ModifyNetworkInterfaceAttribute(ctx, &ec2.ModifyNetworkInterfaceAttributeInput{
		Attachment: &types.NetworkInterfaceAttachmentChanges{
			AttachmentId:        nicAttachOp.AttachmentId,
			DeleteOnTermination: aws.Bool(true),
		},
		NetworkInterfaceId: nIfaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to modify the network interface attribute: %v", err)
	}

	logger.Printf("created a network interface %s and attached it to the instance %s", *nIfaceID, instanceID)

	return nIfaceID, nil
}

// Create Elastic IP and attach it to the interface
func (p *awsProvider) createElasticIPforInstance(ctx context.Context, instanceID string, nIfaceID *string) error {
	eipName := fmt.Sprintf("eip-%s", instanceID)

	// Create Elastic IP. Allocate from AWS pool
	allocateAddressInput := &ec2.AllocateAddressInput{
		Domain: types.DomainTypeVpc,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeElasticIp,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(eipName),
					},
				},
			},
		},
	}

	eip, err := p.ec2Client.AllocateAddress(ctx, allocateAddressInput)
	if err != nil {
		return fmt.Errorf("failed to allocate an Elastic IP address: %v", err)
	}

	// Wait for instance to be ready before associating the Elastic IP address
	describeInstanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}
	err = p.waiter.Wait(ctx, describeInstanceInput, maxWaitTime)
	if err != nil {
		logger.Printf("failed to wait for the instance to be ready : %v ", err)
		return err
	}

	// Associate the Elastic IP with the instance
	_, err = p.ec2Client.AssociateAddress(ctx, &ec2.AssociateAddressInput{
		AllocationId:       eip.AllocationId,
		AllowReassociation: aws.Bool(true),
		NetworkInterfaceId: nIfaceID,
	})
	if err != nil {
		// Release the Elastic IP address
		_, relErr := p.ec2Client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
			AllocationId: eip.AllocationId,
		})
		if relErr != nil {
			logger.Printf("failed to release the Elastic IP address: %v", relErr)
		}
		return fmt.Errorf("failed to associate an Elastic IP address with the instance: %v", err)
	}

	logger.Printf("associated the Elastic IP address: %s with the instance: %s", *eip.PublicIp, instanceID)

	return nil
}

func (p *awsProvider) deleteElasticIPforInstance(ctx context.Context, instanceID string) error {

	describeAddressInput := &ec2.DescribeAddressesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("instance-id"),
				Values: []string{instanceID},
			},
		},
	}

	// Describe addresses to find the Elastic IP
	describeAddressesOutput, err := p.ec2Client.DescribeAddresses(ctx, describeAddressInput)
	if err != nil {
		return fmt.Errorf("failed to describe the Elastic IP addresses: %v for instance: %s", err, instanceID)
	}

	if len(describeAddressesOutput.Addresses) == 0 {
		logger.Printf("No Elastic IP addresses found for instance: %s", instanceID)
		return nil
	}

	// Find the Elastic IP associated with the given network interface and delete it
	for _, addr := range describeAddressesOutput.Addresses {
		//if addr.NetworkInterfaceId != nil && *addr.NetworkInterfaceId == *nIfaceId {
		if addr.InstanceId != nil && *addr.InstanceId == instanceID {

			// Disassociate the Elastic IP address
			_, err = p.ec2Client.DisassociateAddress(ctx, &ec2.DisassociateAddressInput{
				AssociationId: addr.AssociationId,
			})
			if err != nil {
				return fmt.Errorf("failed to disassociate the Elastic IP address: %v", err)
			}

			// Release the Elastic IP address
			_, err = p.ec2Client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
				AllocationId: addr.AllocationId,
			})
			if err != nil {
				return fmt.Errorf("failed to release the Elastic IP address: %v", err)
			}
			logger.Printf("released the Elastic IP address: %s for instance: %s", *addr.PublicIp, instanceID)
		}
	}

	return nil
}

func (p *awsProvider) getDeviceNameAndSize(imageID string) (string, int32, error) {
	// Add describe images input
	describeImagesInput := &ec2.DescribeImagesInput{
		ImageIds: []string{imageID},
	}

	// Add describe images output
	describeImagesOutput, err := p.ec2Client.DescribeImages(context.Background(), describeImagesInput)
	if err != nil {
		logger.Printf("failed to describe image %s: %v", imageID, err)
		return "", 0, err
	}

	if describeImagesOutput == nil || len(describeImagesOutput.Images) == 0 {
		return "", 0, errImageDetailsFailed
	}

	// Get the device name
	deviceName := describeImagesOutput.Images[0].RootDeviceName

	// Check if the device name is empty
	if deviceName == nil || *deviceName == "" {
		return "", 0, errDeviceNameEmpty
	}

	// Get the device size if it is set
	deviceSize := describeImagesOutput.Images[0].BlockDeviceMappings[0].Ebs.VolumeSize

	if deviceSize == nil {
		logger.Printf("image %s device size not set", imageID)
		return *deviceName, 0, nil
	}

	logger.Printf("image %s: device name=[%s], size=[%d]", imageID, *deviceName, *deviceSize)

	return *deviceName, *deviceSize, nil
}

func (p *awsProvider) ListInstances(ctx context.Context, input provider.ListInstancesInput) ([]*provider.Instance, error) {
	if input.ClusterUID == "" {
		return nil, fmt.Errorf("cluster UID is required")
	}

	describeInput := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:" + provider.ClusterUIDTagKey),
				Values: []string{input.ClusterUID},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"pending", "running", "stopping", "stopped"},
			},
		},
	}

	var instances []*provider.Instance
	for {
		result, err := p.ec2Client.DescribeInstances(ctx, describeInput)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		for _, reservation := range result.Reservations {
			for _, inst := range reservation.Instances {
				if inst.InstanceId == nil {
					continue
				}
				name := ""
				for _, tag := range inst.Tags {
					if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
						name = *tag.Value
						break
					}
				}
				instances = append(instances, &provider.Instance{
					ID:        *inst.InstanceId,
					Name:      name,
					CreatedAt: aws.ToTime(inst.LaunchTime),
				})
			}
		}

		if result.NextToken == nil {
			break
		}
		describeInput.NextToken = result.NextToken
	}

	logger.Printf("ListInstances: found %d instances for cluster UID %s", len(instances), input.ClusterUID)
	return instances, nil
}
