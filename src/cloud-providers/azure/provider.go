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
	"github.com/avast/retry-go/v4"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
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

func NewProvider(config *Config) (provider.Provider, error) {

	logger.Printf("azure config %+v", config.Redact())

	azureClient, err := NewAzureClient(*config)
	if err != nil {
		logger.Printf("creating azure client: %v", err)
		return nil, err
	}

	provider := &azureProvider{
		azureClient:   azureClient,
		serviceConfig: config,
	}

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
		Tags: p.getResourceTags(),
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

// Method to update the network Interface with the public IP
func (p *azureProvider) attachPublicIpAddr(ctx context.Context, vmNic *armnetwork.Interface,
	publicIpAddr *armnetwork.PublicIPAddress) error {
	nicClient, err := armnetwork.NewInterfacesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating network interface client: %w", err)
	}

	// Update the network interface with the public IP
	vmNic.Properties.IPConfigurations[0].Properties.PublicIPAddress = &armnetwork.PublicIPAddress{
		ID: publicIpAddr.ID,
	}

	pollerResponse, err := nicClient.BeginCreateOrUpdate(ctx, p.serviceConfig.ResourceGroupName, *vmNic.Name, *vmNic, nil)
	if err != nil {
		return fmt.Errorf("beginning update of network interface: %w", err)
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("polling network interface update: %w", err)
	}

	return nil
}

// Method to create a public IP
func (p *azureProvider) createPublicIP(ctx context.Context, publicIPName string) (*armnetwork.PublicIPAddress, error) {
	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, fmt.Errorf("creating public IP client: %w", err)
	}

	parameters := armnetwork.PublicIPAddress{
		Name:     to.Ptr(publicIPName),
		Location: to.Ptr(p.serviceConfig.Region),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameBasic),
			Tier: to.Ptr(armnetwork.PublicIPAddressSKUTierRegional),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAddressVersion:   to.Ptr(armnetwork.IPVersionIPv4),
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			// Delete the public IP when the associated VM is deleted
			DeleteOption: to.Ptr(armnetwork.DeleteOptionsDelete),
		},

		Tags: p.getResourceTags(),
	}

	pollerResponse, err := publicIPClient.BeginCreateOrUpdate(ctx, p.serviceConfig.ResourceGroupName, publicIPName, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning creation or update of public IP: %w", err)
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("polling public IP creation: %w", err)
	}

	return &resp.PublicIPAddress, nil

}

// Method to delete the public IP
func (p *azureProvider) deletePublicIP(ctx context.Context, publicIpAddrName string) error {
	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating public IP client: %w", err)
	}

	rg := p.serviceConfig.ResourceGroupName

	// retry with exponential backoff
	err = retry.Do(func() error {
		pollerResponse, err := publicIPClient.BeginDelete(ctx, rg, publicIpAddrName, nil)
		if err != nil {
			return fmt.Errorf("beginning public IP deletion: %w", err)
		}
		_, err = pollerResponse.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("waiting for public IP deletion: %w", err)
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
		logger.Printf("deleting network interface (%s): %s", publicIpAddrName, err)
		return err
	}

	logger.Printf("successfully deleted nic (%s)", publicIpAddrName)
	return nil
}

func (p *azureProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
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

	// Create public IP if serviceConfig.UsePublicIP is true
	var publicIpAddr *armnetwork.PublicIPAddress
	var publicIpName string

	if p.serviceConfig.UsePublicIP {
		publicIpName = fmt.Sprintf("%s-ip", instanceName)
		publicIpAddr, err = p.createPublicIP(ctx, publicIpName)
		if err != nil {
			err = fmt.Errorf("creating public IP: %w", err)
			logger.Printf("%v", err)
			return nil, err
		}

		logger.Printf("public IP (%s) created with address: %s", *publicIpAddr.Name, *publicIpAddr.Properties.IPAddress)

		// Attach the public IP to the NIC
		err = p.attachPublicIpAddr(ctx, vmNIC, publicIpAddr)
		if err != nil {
			logger.Printf("error in attaching public IP to the NIC: %v", err)
			// Delete the public IP if attaching fails
			if err := p.deletePublicIP(ctx, publicIpName); err != nil {
				logger.Printf("deleting public IP (%s): %s", publicIpName, err)
			}
			return nil, err
		}

	}

	vmParameters, err := p.getVMParameters(instanceSize, diskName, cloudConfigData, sshBytes, instanceName, vmNIC)
	if err != nil {
		return nil, err
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	result, err := p.create(ctx, vmParameters)
	if err != nil {
		if err := p.deleteDisk(context.Background(), diskName); err != nil {
			logger.Printf("deleting disk (%s): %s", diskName, err)
		}
		if err := p.deleteNetworkInterface(context.Background(), nicName); err != nil {
			logger.Printf("deleting nic async (%s): %s", nicName, err)
		}

		if p.serviceConfig.UsePublicIP {
			if err := p.deletePublicIP(context.Background(), publicIpName); err != nil {
				logger.Printf("deleting public IP (%s): %s", publicIpName, err)
			}
		}

		return nil, fmt.Errorf("Creating instance (%v): %s", result, err)
	}

	instanceID := *result.ID

	ips, err := getIPs(vmNIC)
	if err != nil {
		logger.Printf("getting IPs for the instance : %v ", err)
		return nil, err
	}

	if p.serviceConfig.UsePublicIP && publicIpAddr != nil {
		// Replace the first IP address with the public IP address
		ip, err := netip.ParseAddr(*publicIpAddr.Properties.IPAddress)
		if err != nil {
			return nil, fmt.Errorf("parsing pod node public IP %q: %w", *publicIpAddr.Properties.IPAddress, err)
		}
		logger.Printf("publicIP=%s", ip.String())
		ips[0] = ip

	}

	instance := &provider.Instance{
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

func (p *azureProvider) deleteNetworkInterface(ctx context.Context, nicName string) error {
	nicClient, err := armnetwork.NewInterfacesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating network interface client: %w", err)
	}
	rg := p.serviceConfig.ResourceGroupName

	// retry with exponential backoff
	err = retry.Do(func() error {
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
		logger.Printf("deleting network interface (%s): %s", nicName, err)
		return err
	}

	logger.Printf("successfully deleted nic (%s)", nicName)
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
func (p *azureProvider) selectInstanceType(ctx context.Context, spec provider.InstanceTypeSpec) (string, error) {

	return provider.SelectInstanceTypeToUse(spec, p.serviceConfig.InstanceSizeSpecList, p.serviceConfig.InstanceSizes, p.serviceConfig.Size)
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
	var instanceSizeSpecList []provider.InstanceTypeSpec

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
				instanceSizeSpecList = append(instanceSizeSpecList, provider.InstanceTypeSpec{InstanceType: *vmSize.Name, VCPUs: int64(*vmSize.NumberOfCores), Memory: int64(*vmSize.MemoryInMB)})
			}
		}
	}

	// Sort the InstanceSizeSpecList by Memory and update the serviceConfig
	p.serviceConfig.InstanceSizeSpecList = provider.SortInstanceTypesOnMemory(instanceSizeSpecList)
	logger.Printf("instanceSizeSpecList (%v)", p.serviceConfig.InstanceSizeSpecList)
	return nil
}

func (p *azureProvider) getResourceTags() map[string]*string {
	tags := map[string]*string{}

	// Add custom tags from serviceConfig.Tags
	for k, v := range p.serviceConfig.Tags {
		tags[k] = to.Ptr(v)
	}
	return tags
}

func (p *azureProvider) getVMParameters(instanceSize, diskName, cloudConfig string, sshBytes []byte, instanceName string, vmNIC *armnetwork.Interface) (*armcompute.VirtualMachine, error) {
	userDataB64 := base64.StdEncoding.EncodeToString([]byte(cloudConfig))

	// Azure limits the base64 encrypted userData to 64KB.
	// Ref: https://learn.microsoft.com/en-us/azure/virtual-machines/user-data
	// If the b64EncData is greater than 64KB then return an error
	if len(userDataB64) > 64*1024 {
		return nil, fmt.Errorf("base64 encoded userData is greater than 64KB")
	}
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
				SecureBootEnabled: to.Ptr(p.serviceConfig.EnableSecureBoot),
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
			DiagnosticsProfile: &armcompute.DiagnosticsProfile{
				BootDiagnostics: &armcompute.BootDiagnostics{
					Enabled: to.Ptr(true),
				},
			},
			UserData: to.Ptr(userDataB64),
		},
		Tags: p.getResourceTags(),
	}

	return &vmParameters, nil
}
