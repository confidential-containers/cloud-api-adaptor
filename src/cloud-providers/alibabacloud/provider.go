// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecs "github.com/alibabacloud-go/ecs-20140526/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	vpc "github.com/alibabacloud-go/vpc-20160428/v6/client"
	"github.com/aliyun/credentials-go/credentials"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/alibabacloud] ", log.LstdFlags|log.Lmsgprefix)

const (
	maxInstanceNameLen = 63

	EnvRoleArn         = "ALIBABA_CLOUD_ROLE_ARN"
	EnvOidcProviderArn = "ALIBABA_CLOUD_OIDC_PROVIDER_ARN"
	EnvOidcTokenFile   = "ALIBABA_CLOUD_OIDC_TOKEN_FILE"
)

// Make ecsClient a mockable interface
type ecsClient interface {
	RunInstances(
		params *ecs.RunInstancesRequest) (*ecs.RunInstancesResponse, error)
	DeleteInstance(
		params *ecs.DeleteInstanceRequest) (*ecs.DeleteInstanceResponse, error)
	// Describe InstanceTypes
	DescribeInstanceTypes(
		params *ecs.DescribeInstanceTypesRequest) (*ecs.DescribeInstanceTypesResponse, error)
	// Describe InstanceAttribute
	DescribeInstanceAttribute(
		params *ecs.DescribeInstanceAttributeRequest) (*ecs.DescribeInstanceAttributeResponse, error)
	// Create NIC
	CreateNetworkInterface(
		params *ecs.CreateNetworkInterfaceRequest) (*ecs.CreateNetworkInterfaceResponse, error)
	// Delete NIC
	DeleteNetworkInterface(
		params *ecs.DeleteNetworkInterfaceRequest) (*ecs.DeleteNetworkInterfaceResponse, error)
	// Atttach NIC
	AttachNetworkInterface(
		params *ecs.AttachNetworkInterfaceRequest) (*ecs.AttachNetworkInterfaceResponse, error)
	// Modify NIC Attributes
	ModifyNetworkInterfaceAttribute(
		params *ecs.ModifyNetworkInterfaceAttributeRequest) (*ecs.ModifyNetworkInterfaceAttributeResponse, error)
	// Describe NIC
	DescribeNetworkInterfaceAttribute(
		params *ecs.DescribeNetworkInterfaceAttributeRequest) (*ecs.DescribeNetworkInterfaceAttributeResponse, error)
}

// Make ecsClient a mockable interface
type vpcClient interface {
	DescribeVSwitchAttributes(
		params *vpc.DescribeVSwitchAttributesRequest) (*vpc.DescribeVSwitchAttributesResponse, error)
	AllocateEipAddress(
		params *vpc.AllocateEipAddressRequest) (*vpc.AllocateEipAddressResponse, error)
	ReleaseEipAddress(
		params *vpc.ReleaseEipAddressRequest) (*vpc.ReleaseEipAddressResponse, error)
	AssociateEipAddress(
		params *vpc.AssociateEipAddressRequest) (*vpc.AssociateEipAddressResponse, error)
	UnassociateEipAddress(
		params *vpc.UnassociateEipAddressRequest) (*vpc.UnassociateEipAddressResponse, error)
	DescribeEipAddresses(
		params *vpc.DescribeEipAddressesRequest) (*vpc.DescribeEipAddressesResponse, error)
}

type alibabaCloudProvider struct {
	// Make ecsClient a mockable interface
	ecsClient     ecsClient
	vpcClient     vpcClient
	serviceConfig *Config

	// instanceId to instance Resources
	eipsMu sync.Mutex
	eips   map[string]*string
}

func NewProvider(config *Config) (provider.Provider, error) {
	logger.Printf("alibabacloud config: %#v", config.Redact())

	var c openapi.Config
	if len(config.AccessKeyId) == 0 || len(config.SecretKey) == 0 {
		logger.Printf("ALIBABACLOUD_ACCESS_KEY_ID and ALIBABACLOUD_ACCESS_KEY_SECRET not provided, try using ACK RRSA (ALIBABA_CLOUD_ROLE_ARN, ALIBABA_CLOUD_OIDC_PROVIDER_ARN, ALIBABA_CLOUD_OIDC_TOKEN_FILE) to get credential...")
		cred, err := credentials.NewCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to pass authentication of alibaba cloud: %v", err)
		}

		_, err = cred.GetCredential()
		if err != nil {
			return nil, fmt.Errorf("failed to get credential of alibaba cloud: %v", err)
		}

		c = openapi.Config{
			Credential: cred,
			RegionId:   tea.String(config.Region),
		}

	} else {
		logger.Printf("Use ALIBABACLOUD_ACCESS_KEY_ID and ALIBABACLOUD_ACCESS_KEY_SECRET as credential")
		c = openapi.Config{
			AccessKeyId:     tea.String(config.AccessKeyId),
			AccessKeySecret: tea.String(config.SecretKey),
			RegionId:        tea.String(config.Region),
		}
	}

	ecsClient, err := ecs.NewClient(&c)
	if err != nil {
		return nil, fmt.Errorf("create alibaba cloud ecs client error: %v", err)
	}

	vpcClient, err := vpc.NewClient(&c)
	if err != nil {
		return nil, fmt.Errorf("create alibaba cloud vpc client error: %v", err)
	}

	provider := &alibabaCloudProvider{
		ecsClient:     ecsClient,
		serviceConfig: config,
		vpcClient:     vpcClient,
		eips:          make(map[string]*string),
	}

	if err = provider.updateInstanceTypeSpecList(); err != nil {
		return nil, fmt.Errorf("failed to update instance type spec list: %v", err)
	}

	if err = provider.updateVpcId(); err != nil {
		return nil, fmt.Errorf("failed to get vpc id: %v", err)
	}

	return provider, nil
}

func (p *alibabaCloudProvider) getIPs(instanceId string, ecsClient ecsClient) ([]netip.Addr, error) {
	var podNodeIPs []netip.Addr

	// describe instnace to get the private IP address
	var privateIPs []*string
	err := p.waitUntilTimeout(time.Duration(time.Second*15), func() (bool, error) {
		req := &ecs.DescribeInstanceAttributeRequest{
			InstanceId: tea.String(instanceId),
		}
		resp, err := p.ecsClient.DescribeInstanceAttribute(req)
		if err == nil {
			// sometimes the Ip address is still not initialized
			if len(resp.Body.VpcAttributes.PrivateIpAddress.IpAddress) > 0 {
				privateIPs = resp.Body.VpcAttributes.PrivateIpAddress.IpAddress
				return true, nil
			}
			return false, nil
		}

		if *err.(*tea.SDKError).StatusCode == 404 {
			return false, nil
		}

		if strings.HasSuffix(*err.(*tea.SDKError).Message, "is not ready") {
			return false, nil
		}

		return false, fmt.Errorf("failed to describe instance %s: %v", instanceId, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait for instance %s to be ready: %v", instanceId, err)
	}

	// Use the VPC private IP address as the pod node IP
	for i, addr := range privateIPs {
		ip, err := netip.ParseAddr(*addr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod node IP %s: %w", *addr, err)
		}
		podNodeIPs = append(podNodeIPs, ip)
		logger.Printf("podNodeIP[%d]=%s", i, ip.String())
	}

	return podNodeIPs, nil
}

func (p *alibabaCloudProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (instance *provider.Instance, err error) {
	// Public IP address
	var publicIPAddr *netip.Addr

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	//Convert userData to base64
	b64EncData := base64.StdEncoding.EncodeToString([]byte(cloudConfigData))

	instanceType, err := p.selectInstanceType(ctx, spec)
	if err != nil {
		return nil, err
	}

	tags := make([]*ecs.RunInstancesRequestTag, 0)
	for k, v := range p.serviceConfig.Tags {
		tags = append(tags, &ecs.RunInstancesRequestTag{
			Key:   tea.String(k),
			Value: tea.String(v),
		})
	}

	var req *ecs.RunInstancesRequest

	if spec.Image != "" {
		logger.Printf("Choosing %s from annotation as the ECS Image for the PodVM image", spec.Image)
		p.serviceConfig.ImageId = spec.Image
	}

	securityGroupIds := []*string{}
	for _, v := range p.serviceConfig.SecurityGroupIds {
		securityGroupIds = append(securityGroupIds, tea.String(v))
	}

	req = &ecs.RunInstancesRequest{
		RegionId:         tea.String(p.serviceConfig.Region),
		MinAmount:        tea.Int32(1),
		Amount:           tea.Int32(1),
		ImageId:          tea.String(p.serviceConfig.ImageId),
		InstanceType:     tea.String(instanceType),
		SecurityGroupIds: securityGroupIds,
		VSwitchId:        tea.String(p.serviceConfig.VswitchId),
		UserData:         tea.String(b64EncData),
		Tag:              tags,
		InstanceName:     tea.String(instanceName),
	}
	if p.serviceConfig.KeyName != "" {
		req.KeyPairName = tea.String(p.serviceConfig.KeyName)
	}

	// Auto assign public IP address if UsePublicIP is set
	if p.serviceConfig.UsePublicIP {
		// Auto-assign public IP
		req.InternetChargeType = tea.String("PayByTraffic")
		req.InternetMaxBandwidthOut = tea.Int32(100)
	}

	if !p.serviceConfig.DisableCVM {
		//  Add IntelTdx Cpu options to the instance
		req.SecurityOptions = &ecs.RunInstancesRequestSecurityOptions{
			ConfidentialComputingMode: tea.String("TDX"),
		}
	}

	// Add block device mappings to the instance to set the root volume size
	if p.serviceConfig.SystemDiskSize > 0 {
		req.ImageId = tea.String(p.serviceConfig.ImageId)
		req.SystemDisk = &ecs.RunInstancesRequestSystemDisk{
			Size:     tea.String(strconv.Itoa(p.serviceConfig.SystemDiskSize)),
			Category: tea.String("cloud_essd"),
		}
		logger.Printf("Setting the SystemDisk size to %d GiB with ImageId %s", p.serviceConfig.SystemDiskSize, p.serviceConfig.ImageId)
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	result, err := p.ecsClient.RunInstances(req)
	if err != nil {
		return nil, fmt.Errorf("creating instance (%v) returned error: %s", result, err)
	}

	instanceID := *result.Body.InstanceIdSets.InstanceIdSet[0]
	logger.Printf("created an instance %s for sandbox %s", instanceID, sandboxID)

	// Create partial instance to return on error (allows caller to cleanup)
	instance = &provider.Instance{
		ID:   instanceID,
		Name: instanceName,
	}

	// Wait instance to create
	err = p.waitUntilTimeout(time.Duration(time.Minute*1), func() (bool, error) {
		req := &ecs.DescribeInstanceAttributeRequest{
			InstanceId: tea.String(instanceID),
		}
		_, err := p.ecsClient.DescribeInstanceAttribute(req)
		if err == nil {
			return true, nil
		}

		if *err.(*tea.SDKError).StatusCode == 404 {
			return false, nil
		}

		if strings.HasSuffix(*err.(*tea.SDKError).Message, "is not ready") {
			return false, nil
		}

		return false, fmt.Errorf("failed to describe instance %s: %v", instanceID, err)
	})
	if err != nil {
		return instance, fmt.Errorf("failed to wait for instance %s to be ready: %v", instanceID, err)
	}
	logger.Printf("Instance %s is ready.", instanceID)

	ips, err := p.getIPs(*result.Body.InstanceIdSets.InstanceIdSet[0], p.ecsClient)
	if err != nil {
		logger.Printf("failed to get IPs for the instance : %v ", err)
		return instance, err
	}

	if p.serviceConfig.UsePublicIP {
		// Get the public IP address of the instance
		publicIPAddr, err = p.getPublicIP(ctx, instanceID)
		if err != nil {
			return instance, err
		}

		// insert the first IP address with the public IP address
		ips = append([]netip.Addr{*publicIPAddr}, ips...)
	}

	// MultiNic flag means that external network connectivity via alibaba cloud is enabled
	// we will create another NIC and create an Internet access
	if spec.MultiNic {
		logger.Println("External network connectivity is enabled, trying to setup another NIC with Internet Access.")
		nIfaceId, err := p.createAddonNICforInstance(instanceID)
		if err != nil {
			return instance, fmt.Errorf("failed to create NIC: %w", err)
		}

		if p.serviceConfig.UsePublicIP {
			eipId, _, err := p.createEipInstance()
			if err != nil {
				return instance, fmt.Errorf("failed to create EIP instance: %w", err)
			}

			p.eipsMu.Lock()
			p.eips[instanceID] = eipId
			p.eipsMu.Unlock()

			err = p.bindEipToNic(eipId, nIfaceId)
			if err != nil {
				return instance, fmt.Errorf("failed to bind Eip %s to NIC %s: %w", *eipId, *nIfaceId, err)
			}
		}

	}

	instance.IPs = ips

	return instance, nil
}

func (p *alibabaCloudProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	logger.Printf("Deleting instance (%s)", instanceID)
	err := p.waitUntilTimeout(time.Duration(time.Second*30), func() (bool, error) {
		req := ecs.DeleteInstanceRequest{
			InstanceId: tea.String(instanceID),
			Force:      tea.Bool(true),
		}
		resp, err := p.ecsClient.DeleteInstance(&req)
		if err != nil {
			if *err.(*tea.SDKError).Code == "IncorrectInstanceStatus" {
				logger.Printf("instance %s is not in the correct state to be deleted, retrying", instanceID)
				return false, nil
			}

			if *err.(*tea.SDKError).Code == "InvalidInstanceId.NotFound" {
				logger.Printf("instance %s is not found", instanceID)
				return true, nil
			}

			logger.Printf("failed to delete an instance: %v and the response is %v", err, resp)
			return false, fmt.Errorf("failed to delete an instance %s: %+v", instanceID, err)
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to delete an instance %s", instanceID)
	}

	logger.Printf("Deleted an instance %s", instanceID)

	p.eipsMu.Lock()
	eipId := p.eips[instanceID]
	delete(p.eips, instanceID)
	p.eipsMu.Unlock()

	if eipId != nil {
		err := p.deleteEipInstance(eipId)
		if err != nil {
			logger.Printf("delete EIP %s failed: %v", *eipId, err)
		}
	}

	return nil
}

func (p *alibabaCloudProvider) Teardown() error {
	return nil
}

func (p *alibabaCloudProvider) ConfigVerifier() error {
	ImageId := p.serviceConfig.ImageId
	if len(ImageId) == 0 {
		return fmt.Errorf("ImageId is empty")
	}
	return nil
}

func (p *alibabaCloudProvider) updateVpcId() error {
	request := &vpc.DescribeVSwitchAttributesRequest{
		VSwitchId: tea.String(p.serviceConfig.VswitchId),
		RegionId:  tea.String(p.serviceConfig.Region),
	}

	response, err := p.vpcClient.DescribeVSwitchAttributes(request)
	if err != nil {
		return fmt.Errorf("failed to describe the vswitch attribute: %v", err)
	}

	p.serviceConfig.VpcId = *response.Body.VpcId
	return nil
}

// Add SelectInstanceType method to select an instance type based on the memory and vcpu requirements
func (p *alibabaCloudProvider) selectInstanceType(_ context.Context, spec provider.InstanceTypeSpec) (string, error) {

	return provider.SelectInstanceTypeToUse(spec, p.serviceConfig.InstanceTypeSpecList, p.serviceConfig.InstanceTypes, p.serviceConfig.InstanceType)
}

// Add a method to populate InstanceTypeSpecList for all the instanceTypes
func (p *alibabaCloudProvider) updateInstanceTypeSpecList() error {
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

// Add a method to retrieve cpu, memory, and storage from the instance type
func (p *alibabaCloudProvider) getInstanceTypeInformation(instanceType string) (vcpu int64, memory int64,
	gpuCount int64, err error) {

	types := []string{instanceType}
	// Get the instance type information from the instance type using ECS API
	req := &ecs.DescribeInstanceTypesRequest{
		InstanceTypes: tea.StringSlice(types),
	}
	// Get the instance type information from the instance type using AlibabaCloud API
	result, err := p.ecsClient.DescribeInstanceTypes(req)
	if err != nil {
		return 0, 0, 0, err
	}

	// Get the vcpu, memory and gpu from the result
	if len(result.Body.InstanceTypes.InstanceType) > 0 {
		instanceInfo := result.Body.InstanceTypes.InstanceType[0]
		vcpu = int64(*instanceInfo.CpuCoreCount)
		memory = int64(*instanceInfo.MemorySize * 1024)

		return vcpu, memory, int64(*instanceInfo.GPUAmount), nil
	}
	return 0, 0, 0, fmt.Errorf("instance type %s not found", instanceType)

}

// Add a method to get public IP address of the instance
// Take the instance id as an argument
// Return the public IP address as a string
func (p *alibabaCloudProvider) getPublicIP(_ context.Context, instanceID string) (*netip.Addr, error) {
	var err error
	var resp *ecs.DescribeInstanceAttributeResponse

	err = p.waitUntilTimeout(time.Duration(time.Second*20), func() (bool, error) {
		req := &ecs.DescribeInstanceAttributeRequest{
			InstanceId: tea.String(instanceID),
		}
		resp, err = p.ecsClient.DescribeInstanceAttribute(req)
		if err != nil {
			logger.Printf("failed to describe instance %q: %v", instanceID, err)
			return false, nil
		}

		if len(resp.Body.PublicIpAddress.IpAddress) == 0 {
			logger.Printf("instance %v with 0 ips", instanceID)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get public ip of instance %v: %v", instanceID, err)
	}

	res, err := netip.ParseAddr(*resp.Body.PublicIpAddress.IpAddress[0])
	return &res, err
}

func (p *alibabaCloudProvider) waitUntilTimeout(timeout time.Duration, judgementFunc func() (bool, error)) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}

	remainingTime := timeout
	var attempt int64

	for {
		attempt++
		start := time.Now()
		if time.Since(start) > timeout {
			return errors.New("request timeout")
		}

		condition, err := judgementFunc()
		if err != nil {
			return err
		}
		if condition {
			return nil
		}

		remainingTime -= time.Since(start)
		if remainingTime < 0 {
			break
		}
		time.Sleep(time.Duration(1 * time.Second))
	}

	return fmt.Errorf("exceeded max wait time to wait")
}

// Create a NIC and attach it to the instance
// Note that the NIC's SecurityGroupId will be the first on of the ECS Instances
func (p *alibabaCloudProvider) createAddonNICforInstance(instanceID string) (nIfaceId *string, err error) {
	networkInterfaceName := fmt.Sprintf("peerpod-nic-%s", instanceID)
	description := ""
	createNetworkInterfaceRequest := &ecs.CreateNetworkInterfaceRequest{
		RegionId:             tea.String(p.serviceConfig.Region),
		VSwitchId:            tea.String(p.serviceConfig.VswitchId),
		SecurityGroupId:      tea.String(p.serviceConfig.SecurityGroupIds[0]),
		NetworkInterfaceName: tea.String(networkInterfaceName),
		Description:          tea.String(description),
	}

	tried := 0
	for {
		tried++
		if tried > 5 {
			return nil, fmt.Errorf("failed to create a network interface after 5 tries: %v", err)
		}
		nic, err := p.ecsClient.CreateNetworkInterface(createNetworkInterfaceRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to create a network interface: %v", err)
		}

		// Due to https://www.alibabacloud.com/help/en/ecs/developer-reference/api-ecs-2014-05-26-createnetworkinterface
		// if the `NetworkInterfaceId` is nil, we need to retry
		if nic.Body.NetworkInterfaceId == nil {
			continue
		}

		nIfaceId = nic.Body.NetworkInterfaceId
		break
	}

	err = p.waitUntilTimeout(time.Duration(time.Minute*1), func() (bool, error) {
		request := &ecs.DescribeInstanceAttributeRequest{
			InstanceId: tea.String(instanceID),
		}
		response, err := p.ecsClient.DescribeInstanceAttribute(request)
		if err != nil {
			return false, fmt.Errorf("failed to get ECS status: %v", err)
		}
		if *response.Body.Status == "Running" {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach NIC to ECS: %v", err)
	}

	// The instance is already created successfully, so we directly attach the NIC to it!
	attachNetworkInterfaceRequest := &ecs.AttachNetworkInterfaceRequest{
		InstanceId:         tea.String(instanceID),
		NetworkInterfaceId: nIfaceId,
		RegionId:           tea.String(p.serviceConfig.Region),
	}

	_, err = p.ecsClient.AttachNetworkInterface(attachNetworkInterfaceRequest)
	if err != nil {
		_, nicDelErr := p.ecsClient.DeleteNetworkInterface(&ecs.DeleteNetworkInterfaceRequest{
			RegionId:           tea.String(p.serviceConfig.Region),
			NetworkInterfaceId: nIfaceId,
		})
		if nicDelErr != nil {
			logger.Printf("failed to delete the network interface: %v", nicDelErr)
		}

		return nil, fmt.Errorf("failed to attach a network interface: %v", err)
	}

	err = p.waitUntilTimeout(time.Duration(time.Minute*1), func() (bool, error) {
		request := &ecs.DescribeNetworkInterfaceAttributeRequest{
			RegionId:           tea.String(p.serviceConfig.Region),
			NetworkInterfaceId: nIfaceId,
		}
		response, err := p.ecsClient.DescribeNetworkInterfaceAttribute(request)
		if err != nil {
			return false, fmt.Errorf("failed to get NIC status: %v", err)
		}
		if *response.Body.Status == "InUse" {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach NIC to ECS: %v", err)
	}

	modifyNetworkInterfaceAttributeRequest := ecs.ModifyNetworkInterfaceAttributeRequest{
		RegionId:           tea.String(p.serviceConfig.Region),
		NetworkInterfaceId: nIfaceId,
		DeleteOnRelease:    tea.Bool(true),
	}
	_, err = p.ecsClient.ModifyNetworkInterfaceAttribute(&modifyNetworkInterfaceAttributeRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to modify the network interface attribute: %v", err)
	}

	logger.Printf("created a network interface %s and attached it to the instance %s", *nIfaceId, instanceID)

	return nIfaceId, nil
}

func (p *alibabaCloudProvider) createEipInstance() (*string, *string, error) {
	logger.Println("Allocating an EIP...")
	req := vpc.AllocateEipAddressRequest{
		RegionId:    tea.String(p.serviceConfig.Region),
		Description: tea.String("Peerpod External Network EIP"),
	}

	res, err := p.vpcClient.AllocateEipAddress(&req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to allocate EIP: %v", err)
	}

	err = p.waitUntilTimeout(time.Duration(time.Minute*1), func() (bool, error) {
		request := &vpc.DescribeEipAddressesRequest{
			RegionId:     tea.String(p.serviceConfig.Region),
			AllocationId: res.Body.AllocationId,
		}
		response, err := p.vpcClient.DescribeEipAddresses(request)
		if err != nil {
			return false, fmt.Errorf("failed to get EIP status: %v", err)
		}

		if len(response.Body.EipAddresses.EipAddress) == 0 {
			return false, nil
		}

		if *response.Body.EipAddresses.EipAddress[0].Status == "Available" {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create EIP: %v", err)
	}

	logger.Printf("Eip %s allocated", *res.Body.EipAddress)
	return res.Body.AllocationId, res.Body.EipAddress, nil
}

func (p *alibabaCloudProvider) deleteEipInstance(eipId *string) error {
	logger.Printf("Unbinding EIP %s...", *eipId)
	unbindReq := vpc.UnassociateEipAddressRequest{
		RegionId:     tea.String(p.serviceConfig.Region),
		AllocationId: eipId,
	}

	_, err := p.vpcClient.UnassociateEipAddress(&unbindReq)
	if err != nil {
		return fmt.Errorf("failed to unbind EIP: %v", err)
	}

	err = p.waitUntilTimeout(time.Duration(time.Second*30), func() (bool, error) {
		request := &vpc.DescribeEipAddressesRequest{
			RegionId:     tea.String(p.serviceConfig.Region),
			AllocationId: eipId,
		}
		response, err := p.vpcClient.DescribeEipAddresses(request)
		if err != nil {
			return false, fmt.Errorf("failed to get EIP status: %v", err)
		}

		if len(response.Body.EipAddresses.EipAddress) == 0 {
			return false, nil
		}

		if *response.Body.EipAddresses.EipAddress[0].Status == "Available" {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to unbind EIP: %v", err)
	}

	deleteReq := vpc.ReleaseEipAddressRequest{
		RegionId:     tea.String(p.serviceConfig.Region),
		AllocationId: eipId,
	}

	logger.Printf("Deleting EIP %s...", *eipId)
	_, err = p.vpcClient.ReleaseEipAddress(&deleteReq)
	if err != nil {
		return fmt.Errorf("failed to delete EIP: %v", err)
	}

	err = p.waitUntilTimeout(time.Duration(time.Second*30), func() (bool, error) {
		request := &vpc.DescribeEipAddressesRequest{
			RegionId:     tea.String(p.serviceConfig.Region),
			AllocationId: eipId,
		}
		response, err := p.vpcClient.DescribeEipAddresses(request)
		if err != nil {
			return false, fmt.Errorf("failed to get EIP status: %v", err)
		}

		if len(response.Body.EipAddresses.EipAddress) == 0 {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to delete EIP: %v", err)
	}

	logger.Printf("Eip %s deleted", *eipId)
	return nil
}

func (p *alibabaCloudProvider) bindEipToNic(eipId *string, nicId *string) error {
	logger.Printf("Binding EIP %s to NIC %s...\n", *eipId, *nicId)
	req := vpc.AssociateEipAddressRequest{
		RegionId:     tea.String(p.serviceConfig.Region),
		AllocationId: eipId,
		InstanceId:   nicId,
		InstanceType: tea.String("NetworkInterface"),
	}

	_, err := p.vpcClient.AssociateEipAddress(&req)
	if err != nil {
		return fmt.Errorf("failed to bind EIP: %v", err)
	}

	err = p.waitUntilTimeout(time.Duration(time.Minute*1), func() (bool, error) {
		request := &vpc.DescribeEipAddressesRequest{
			RegionId:     tea.String(p.serviceConfig.Region),
			AllocationId: eipId,
		}
		response, err := p.vpcClient.DescribeEipAddresses(request)
		if err != nil {
			return false, fmt.Errorf("failed to get NIC EIP binding status: %v", err)
		}

		if len(response.Body.EipAddresses.EipAddress) == 0 {
			return false, nil
		}

		if *response.Body.EipAddresses.EipAddress[0].Status == "InUse" {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to bind EIP %s to NIC %s: %v", *eipId, *nicId, err)
	}

	logger.Printf("Bound EIP %s to NIC %s successfully", *eipId, *nicId)
	return nil
}
