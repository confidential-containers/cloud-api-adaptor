// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/avast/retry-go/v4"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/azure] ", log.LstdFlags|log.Lmsgprefix)
var errNotReady = errors.New("address not ready")
var errNotFound = errors.New("VM name not found")

const (
	maxInstanceNameLen = 63
)

type azureProvider struct {
	azureClient   azcore.TokenCredential
	serviceConfig *Config
}

// TODO: Add support for managed K8s - AKS and ARO
// AKS and ARO creates a separate resource group for the cluster resources and current code needs to be adapted to handle retrieving the cluster resource group and related settings
func (p *azureProvider) fetchConfigMapValues() error {
	config := p.serviceConfig

	if config.ResourceGroupName == "" {
		err := p.GetResourceGroup()
		if err != nil {
			return fmt.Errorf("getting ResourceGroup from azure: %w", err)
		}
	}

	if config.SecurityGroupId == "" {
		err := p.GetNSG(config.ResourceGroupName)
		if err != nil {
			return fmt.Errorf("getting NSG_ID from azure: %w", err)
		}
	}

	if config.SubnetId == "" {
		err := p.GetVirtualNetwork(config.ResourceGroupName)
		if err != nil {
			return fmt.Errorf("getting SubnetID from azure: %w", err)
		}
	}

	return nil
}

func NewProvider(config *Config) (cloud.Provider, error) {

	// Requires config.TenantId, config.ClientId and config.ClientSecret to be set
	azureClient, err := NewAzureClient(*config)
	if err != nil {
		logger.Printf("creating azure client: %v", err)
		return nil, err
	}

	provider := &azureProvider{
		azureClient:   azureClient,
		serviceConfig: config,
	}

	// Uses Azure sdk to get config.ResourceGroupName, config.SecurityGroupId,
	// config.ImageId and config.SubnetId
	if err = provider.fetchConfigMapValues(); err != nil {
		return nil, err
	}

	logger.Printf("azure config %+v", config.Redact())

	// Uses Azure sdk to get config.InstanceTypeSpecList
	if err = provider.updateInstanceSizeSpecList(); err != nil {
		return nil, err
	}

	return provider, nil
}

func getIPs(nic *armnetwork.Interface) ([]netip.Addr, error) {
	var podNodeIPs []netip.Addr

	for i, ipc := range nic.Properties.IPConfigurations {
		addr := ipc.Properties.PrivateIPAddress

		if addr == nil || *addr == "" || *addr == "0.0.0.0" {
			return nil, errNotReady
		}

		ip, err := netip.ParseAddr(*addr)
		if err != nil {
			return nil, fmt.Errorf("parsing pod node IP %q: %w", *addr, err)
		}

		podNodeIPs = append(podNodeIPs, ip)
		logger.Printf("podNodeIP[%d]=%s", i, ip.String())
	}

	return podNodeIPs, nil
}

func (p *azureProvider) create(ctx context.Context, parameters *armcompute.VirtualMachine) (*armcompute.VirtualMachine, error) {
	vmClient, err := armcompute.NewVirtualMachinesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, fmt.Errorf("creating VM client: %w", err)
	}

	vmName := *parameters.Properties.OSProfile.ComputerName

	pollerResponse, err := vmClient.BeginCreateOrUpdate(ctx, p.serviceConfig.ResourceGroupName, vmName, *parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning VM creation or update: %w", err)
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("waiting for the VM creation: %w", err)
	}

	logger.Printf("created VM successfully: %s", *resp.ID)

	return &resp.VirtualMachine, nil
}

func (p *azureProvider) createNetworkInterface(ctx context.Context, nicName string) (*armnetwork.Interface, error) {
	nicClient, err := armnetwork.NewInterfacesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, fmt.Errorf("creating network interfaces client: %w", err)
	}

	parameters := armnetwork.Interface{
		Location: to.Ptr(p.serviceConfig.Region),
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr(fmt.Sprintf("%s-ipConfig", nicName)),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
						Subnet: &armnetwork.Subnet{
							ID: to.Ptr(p.serviceConfig.SubnetId),
						},
					},
				},
			},
		},
	}

	if p.serviceConfig.SecurityGroupId != "" {
		parameters.Properties.NetworkSecurityGroup = &armnetwork.SecurityGroup{
			ID: to.Ptr(p.serviceConfig.SecurityGroupId),
		}
	}

	pollerResponse, err := nicClient.BeginCreateOrUpdate(ctx, p.serviceConfig.ResourceGroupName, nicName, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning creation or update of network interface: %w", err)
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("polling network interface creation: %w", err)
	}

	return &resp.Interface, nil
}

func (p *azureProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec cloud.InstanceTypeSpec) (*cloud.Instance, error) {

	var b64EncData string

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	// Copy the data in {} after content: from customData
	/*
		#cloud-config
		write_files:
		  - path: /peerpod/daemon.json
		    content: |
		      {
		       ...
			  }
		  - path: other files
	*/

	if !p.serviceConfig.DisableCloudConfig {
		//Convert cloudConfigData to base64
		b64EncData = base64.StdEncoding.EncodeToString([]byte(cloudConfigData))
	} else {
		userData := strings.Split(cloudConfigData, "content: |")[1]
		// Take the data in {} after content: and ignore the rest
		// ToDo: use a regex
		userData = strings.Split(userData, "- path")[0]
		userData = strings.TrimSpace(userData)

		//Convert userData to base64
		b64EncData = base64.StdEncoding.EncodeToString([]byte(userData))
	}

	// Azure limits the base64 encrypted userData to 64KB.
	// Ref: https://learn.microsoft.com/en-us/azure/virtual-machines/user-data
	// If the b64EncData is greater than 64KB then return an error
	if len(b64EncData) > 64*1024 {
		return nil, fmt.Errorf("base64 encoded userData is greater than 64KB")
	}

	instanceSize, err := p.selectInstanceType(ctx, spec)
	if err != nil {
		return nil, err
	}

	diskName := fmt.Sprintf("%s-disk", instanceName)
	nicName := fmt.Sprintf("%s-net", instanceName)

	// require ssh key for authentication on linux
	sshPublicKeyPath := os.ExpandEnv(p.serviceConfig.SSHKeyPath)
	var sshBytes []byte
	if _, err := os.Stat(sshPublicKeyPath); err == nil {
		sshBytes, err = os.ReadFile(sshPublicKeyPath)
		if err != nil {
			err = fmt.Errorf("reading ssh public key file: %w", err)
			logger.Printf("%v", err)
			return nil, err
		}
	} else {
		err = fmt.Errorf("ssh public key: %w", err)
		logger.Printf("%v", err)
		return nil, err
	}

	// Get NIC using subnet and allow ports on the ssh group
	vmNIC, err := p.createNetworkInterface(ctx, nicName)
	if err != nil {
		err = fmt.Errorf("creating VM network interface: %w", err)
		logger.Printf("%v", err)
		return nil, err
	}

	vmParameters, err := p.getVMParameters(instanceSize, diskName, b64EncData, sshBytes, instanceName, vmNIC)
	if err != nil {
		return nil, err
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	result, err := p.create(ctx, vmParameters)
	if err != nil {
		if err := p.deleteDisk(ctx, diskName); err != nil {
			logger.Printf("deleting disk (%s): %s", diskName, err)
		}
		if err := p.deleteNetworkInterfaceAsync(context.Background(), nicName); err != nil {
			logger.Printf("deleting nic async (%s): %s", nicName, err)
		}
		return nil, fmt.Errorf("Creating instance (%v): %s", result, err)
	}

	instanceID := *result.ID

	ips, err := getIPs(vmNIC)
	if err != nil {
		logger.Printf("getting IPs for the instance : %v ", err)
		return nil, err
	}

	instance := &cloud.Instance{
		ID:   instanceID,
		Name: instanceName,
		IPs:  ips,
	}

	return instance, nil
}

func (p *azureProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	vmClient, err := armcompute.NewVirtualMachinesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating VM client: %w", err)
	}

	// instanceID in the form of /subscriptions/<subID>/resourceGroups/<resource_name>/providers/Microsoft.Compute/virtualMachines/<VM_Name>.
	re := regexp.MustCompile(`^/subscriptions/[^/]+/resourceGroups/[^/]+/providers/Microsoft\.Compute/virtualMachines/(.*)$`)
	match := re.FindStringSubmatch(instanceID)
	if len(match) < 1 {
		logger.Print("finding VM name using regexp:", match)
		return errNotFound
	}

	vmName := match[1]

	pollerResponse, err := vmClient.BeginDelete(ctx, p.serviceConfig.ResourceGroupName, vmName, nil)
	if err != nil {
		return fmt.Errorf("beginning VM deletion: %w", err)
	}

	if _, err = pollerResponse.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("waiting for the VM deletion: %w", err)
	}

	logger.Printf("deleted VM successfully: %s", vmName)
	return nil
}

func (p *azureProvider) deleteDisk(ctx context.Context, diskName string) error {
	diskClient, err := armcompute.NewDisksClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating disk client: %w", err)
	}

	pollerResponse, err := diskClient.BeginDelete(ctx, p.serviceConfig.ResourceGroupName, diskName, nil)
	if err != nil {
		return fmt.Errorf("beginning disk deletion: %w", err)
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("waiting for the disk deletion: %w", err)
	}

	logger.Printf("deleted disk successfully: %s", diskName)

	return nil
}

func (p *azureProvider) deleteNetworkInterfaceAsync(ctx context.Context, nicName string) error {
	nicClient, err := armnetwork.NewInterfacesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating network interface client: %w", err)
	}
	rg := p.serviceConfig.ResourceGroupName

	// retry with exponential backoff
	go func() {
		err := retry.Do(func() error {
			pollerResponse, err := nicClient.BeginDelete(ctx, rg, nicName, nil)
			if err != nil {
				return fmt.Errorf("beginning network interface deletion: %w", err)
			}
			_, err = pollerResponse.PollUntilDone(ctx, nil)
			if err != nil {
				return fmt.Errorf("waiting for network interface deletion: %w", err)
			}
			return nil
		},
			retry.Context(ctx),
			retry.Attempts(4),
			retry.Delay(180*time.Second),
			retry.MaxDelay(180*time.Second),
			retry.LastErrorOnly(true),
		)
		if err != nil {
			logger.Printf("deleting network interface in background (%s): %s", nicName, err)
		} else {
			logger.Printf("successfully deleted nic (%s) in background", nicName)
		}
	}()

	return nil
}

func (p *azureProvider) Teardown() error {
	return nil
}

func (p *azureProvider) ConfigVerifier() error {
	ImageId := p.serviceConfig.ImageId
	if len(ImageId) == 0 {
		return fmt.Errorf("ImageId is empty")
	}
	return nil
}

// Add SelectInstanceType method to select an instance type based on the memory and vcpu requirements
func (p *azureProvider) selectInstanceType(ctx context.Context, spec cloud.InstanceTypeSpec) (string, error) {

	return cloud.SelectInstanceTypeToUse(spec, p.serviceConfig.InstanceSizeSpecList, p.serviceConfig.InstanceSizes, p.serviceConfig.Size)
}

// Add a method to populate InstanceSizeSpecList for all the instanceSizes
// available in Azure
func (p *azureProvider) updateInstanceSizeSpecList() error {

	// Create a new instance of the Virtual Machine Sizes client
	vmSizesClient, err := armcompute.NewVirtualMachineSizesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating VM sizes client: %w", err)
	}
	// Get the instance sizes from the service config
	instanceSizes := p.serviceConfig.InstanceSizes

	// If instanceTypes is empty then populate it with the default instance type
	if len(instanceSizes) == 0 {
		instanceSizes = append(instanceSizes, p.serviceConfig.Size)
	}

	// Create a list of instancesizespec
	var instanceSizeSpecList []cloud.InstanceTypeSpec

	// TODO: Is there an optimal method for this?
	// Create NewListPager to iterate over the instance types
	pager := vmSizesClient.NewListPager(p.serviceConfig.Region, &armcompute.VirtualMachineSizesClientListOptions{})

	// Iterate over the page and populate the instanceSizeSpecList for all the instanceSizes
	for pager.More() {
		nextResult, err := pager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("getting next page of VM sizes: %w", err)
		}
		for _, vmSize := range nextResult.VirtualMachineSizeListResult.Value {
			if util.Contains(instanceSizes, *vmSize.Name) {
				instanceSizeSpecList = append(instanceSizeSpecList, cloud.InstanceTypeSpec{InstanceType: *vmSize.Name, VCPUs: int64(*vmSize.NumberOfCores), Memory: int64(*vmSize.MemoryInMB)})
			}
		}
	}

	// Sort the InstanceSizeSpecList by Memory and update the serviceConfig
	p.serviceConfig.InstanceSizeSpecList = cloud.SortInstanceTypesOnMemory(instanceSizeSpecList)
	logger.Printf("instanceSizeSpecList (%v)", p.serviceConfig.InstanceSizeSpecList)
	return nil
}

func (p *azureProvider) getVMParameters(instanceSize, diskName, b64EncData string, sshBytes []byte, instanceName string, vmNIC *armnetwork.Interface) (*armcompute.VirtualMachine, error) {
	var managedDiskParams *armcompute.ManagedDiskParameters
	var securityProfile *armcompute.SecurityProfile
	if !p.serviceConfig.DisableCVM {
		managedDiskParams = &armcompute.ManagedDiskParameters{
			StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
			SecurityProfile: &armcompute.VMDiskSecurityProfile{
				SecurityEncryptionType: to.Ptr(armcompute.SecurityEncryptionTypesVMGuestStateOnly),
			},
		}

		securityProfile = &armcompute.SecurityProfile{
			SecurityType: to.Ptr(armcompute.SecurityTypesConfidentialVM),
			UefiSettings: &armcompute.UefiSettings{
				SecureBootEnabled: to.Ptr(true),
				VTpmEnabled:       to.Ptr(true),
			},
		}
	} else {
		managedDiskParams = &armcompute.ManagedDiskParameters{
			StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
		}

		securityProfile = nil
	}

	imgRef := &armcompute.ImageReference{
		ID: to.Ptr(p.serviceConfig.ImageId),
	}
	if strings.HasPrefix(p.serviceConfig.ImageId, "/CommunityGalleries/") {
		imgRef = &armcompute.ImageReference{
			CommunityGalleryImageID: to.Ptr(p.serviceConfig.ImageId),
		}
	}

	// Add tags to the instance
	tags := map[string]*string{}

	// Add custom tags from serviceConfig.Tags to the instance
	for k, v := range p.serviceConfig.Tags {
		tags[k] = to.Ptr(v)
	}

	vmParameters := armcompute.VirtualMachine{
		Location: to.Ptr(p.serviceConfig.Region),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(instanceSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: imgRef,
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr(diskName),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					Caching:      to.Ptr(armcompute.CachingTypesReadWrite),
					DeleteOption: to.Ptr(armcompute.DiskDeleteOptionTypesDelete),
					ManagedDisk:  managedDiskParams,
				},
			},
			OSProfile: &armcompute.OSProfile{
				AdminUsername: to.Ptr(p.serviceConfig.SSHUserName),
				ComputerName:  to.Ptr(instanceName),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					//TBD: replace with a suitable mechanism to use precreated SSH key
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{{
							Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", p.serviceConfig.SSHUserName)),
							KeyData: to.Ptr(string(sshBytes)),
						}},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: vmNIC.ID,
						Properties: &armcompute.NetworkInterfaceReferenceProperties{
							DeleteOption: to.Ptr(armcompute.DeleteOptionsDelete),
						},
					},
				},
			},
			SecurityProfile: securityProfile,
		},
		// Add tags to the instance
		Tags: tags,
	}

	// If DisableCloudConfig is set to false then set OSProfile.CustomData to b64EncData and
	// armcompute.VirtualMachine.Properties.UserData to nil
	// If DisableCloudConfig is set to true then set armcompute.VirtualMachine.Properties.UserData to b64EncData and
	// OSProfile.CustomData to nil

	if !p.serviceConfig.DisableCloudConfig {
		vmParameters.Properties.OSProfile.CustomData = to.Ptr(b64EncData)
		vmParameters.Properties.UserData = nil
	} else {
		vmParameters.Properties.UserData = to.Ptr(b64EncData)
		vmParameters.Properties.OSProfile.CustomData = nil
	}

	return &vmParameters, nil
}

func _from_user_rg(managedBy string, userRg string) bool {
	parts := strings.Split(managedBy, "/")
	if len(parts) > 5 && parts[3] == "resourceGroups" {
		return parts[4] == userRg
	}
	return false
}

func (p *azureProvider) GetResourceGroup() error {

	resourcesClientFactory, err := armresources.NewClientFactory(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating resource client factory:%w", err)
	}

	rgClient := resourcesClientFactory.NewResourceGroupsClient()
	pager := rgClient.NewListPager(nil)
	found := 0

	for pager.More() {
		nextResult, err := pager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("getting ResourceGroup NextPage: %w", err)
		}
		if nextResult.ResourceGroupListResult.Value != nil {
			for _, ResourceGroup := range nextResult.ResourceGroupListResult.Value {
				if ResourceGroup.ManagedBy != nil {
					rg_matches := _from_user_rg(*ResourceGroup.ManagedBy, p.serviceConfig.ResourceGroupName)
					if rg_matches {
						logger.Printf("ResourceGroup found: %v", (*ResourceGroup.Name))
						if found == 0 {
							p.serviceConfig.ResourceGroupName = *ResourceGroup.Name
							logger.Printf("Using ResourceGroup %v", (*ResourceGroup.Name))
						}
						found++
					}
				}
			}
		}
	}

	if found > 1 {
		logger.Printf("[warning] more than a ResourceGroup found! Defaulting to %v", p.serviceConfig.ResourceGroupName)
	}

	if found == 0 {
		return fmt.Errorf("no ResourceGroup found! Please provide it manually with AZURE_RESOURCE_GROUP")
	}

	return nil
}

func (p *azureProvider) GetVirtualNetwork(resourceGroupName string) error {

	virtualNetworksClient, err := armnetwork.NewVirtualNetworksClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("getting virtualNetworksClient: %w", err)
	}

	found := 0
	virtnet := ""

	virtualNetworksClientNewListPager := virtualNetworksClient.NewListPager(resourceGroupName, nil)
	for virtualNetworksClientNewListPager.More() {
		nextResult, err := virtualNetworksClientNewListPager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("getting virtual networks NextPage: %w", err)
		}
		if nextResult.VirtualNetworkListResult.Value != nil {
			for _, VirtualNetwork := range nextResult.VirtualNetworkListResult.Value {
				logger.Printf("Virtual network found: %v", *VirtualNetwork.Name)
				if found == 0 {
					virtnet = *VirtualNetwork.Name
					logger.Printf("Using Virtual network %v", virtnet)
				}
				found++
			}
		}
	}

	if found > 1 {
		logger.Printf("[warning] more than a Virtual network found! Defaulting to %v", virtnet)
	}

	if found == 0 {
		return fmt.Errorf("no Virtual network found in ResourceGroup %v", resourceGroupName)
	}

	return p.GetSubnetID(resourceGroupName, virtnet)
}

func (p *azureProvider) GetSubnetID(resourceGroupName string, virtualNetworkName string) error {

	subnetsClient, err := armnetwork.NewSubnetsClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("getting NewSubnetsClient: %w", err)
	}

	found := 0

	subnetsClientNewListPager := subnetsClient.NewListPager(resourceGroupName, virtualNetworkName, nil)
	for subnetsClientNewListPager.More() {
		nextResult, err := subnetsClientNewListPager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("getting subnets NextPage: %w", err)
		}
		if nextResult.SubnetListResult.Value != nil {
			for _, Subnet := range nextResult.SubnetListResult.Value {
				logger.Printf("Subnet found: %v", *Subnet.Name)
				if found == 0 {
					p.serviceConfig.SubnetId = *Subnet.ID
					logger.Printf("Using Subnet ID %v", *Subnet.ID)
				}
				found++
			}

		}
	}

	if found > 1 {
		logger.Printf("[warning] more than a Subnet ID found for Virtual network %v! Defaulting to %v", virtualNetworkName, p.serviceConfig.SubnetId)
	}

	if found == 0 {
		return fmt.Errorf("no Subnet ID found in Virtual network %v! Please provide it manually with AZURE_SUBNET_ID", virtualNetworkName)
	}

	return nil
}

func (p *azureProvider) GetNSG(resourceGroupName string) error {

	securityGroupsClient, err := armnetwork.NewSecurityGroupsClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("getting NewSecurityGroupsClient: %w", err)
	}

	found := 0

	securityGroupsClientNewListPager := securityGroupsClient.NewListPager(resourceGroupName, nil)
	for securityGroupsClientNewListPager.More() {
		nextResult, err := securityGroupsClientNewListPager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("getting NSG NextPage: %w", err)
		}
		if nextResult.SecurityGroupListResult.Value != nil {
			for _, SecurityGroup := range nextResult.SecurityGroupListResult.Value {
				logger.Printf("NSG found: %v", *SecurityGroup.ID)
				if found == 0 {
					p.serviceConfig.SecurityGroupId = *SecurityGroup.ID
					logger.Printf("Using NSG %v", *SecurityGroup.ID)
				}
				found++
			}
		}
	}

	if found > 1 {
		logger.Printf("[warning] more than a NSG found! Defaulting to %v", p.serviceConfig.SecurityGroupId)
	}

	if found == 0 {
		return fmt.Errorf("no NSG found in ResourceGroup %v! Please provide it manually with AZURE_NSG_ID", resourceGroupName)
	}

	return nil
}
