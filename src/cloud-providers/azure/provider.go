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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
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

func parseIP(addr string) (*netip.Addr, error) {
	if addr == "" || addr == "0.0.0.0" {
		return nil, errNotReady
	}

	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("parse pod vm IP %q: %w", addr, err)
	}
	return &ip, nil
}

func (p *azureProvider) getIPs(ctx context.Context, vm *armcompute.VirtualMachine) ([]netip.Addr, error) {
	nicClient, err := armnetwork.NewInterfacesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, fmt.Errorf("create network interfaces client: %w", err)
	}
	rgName := p.serviceConfig.ResourceGroupName
	nicRefs := vm.Properties.NetworkProfile.NetworkInterfaces

	var ips []netip.Addr
	var ipcs []*armnetwork.InterfaceIPConfiguration

	for _, nicRef := range nicRefs {
		nicId := *nicRef.ID
		// the last segment of a nic id is the name
		nicName := nicId[strings.LastIndex(nicId, "/")+1:]
		nic, err := nicClient.Get(ctx, rgName, nicName, nil)
		if err != nil {
			return nil, fmt.Errorf("get network interface: %w", err)
		}
		ipcs = append(ipcs, nic.Properties.IPConfigurations...)
	}

	// we add the public ip addresses as first elements, if available
	if p.serviceConfig.UsePublicIP {
		publicIPClient, err := armnetwork.NewPublicIPAddressesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
		if err != nil {
			return nil, fmt.Errorf("create public ip client: %w", err)
		}
		for i, ipc := range ipcs {
			if ipc.Properties.PublicIPAddress == nil {
				continue
			}
			ipID := *ipc.Properties.PublicIPAddress.ID
			// the last segment of a ip id is the name
			ipName := ipID[strings.LastIndex(ipID, "/")+1:]
			publicIP, err := publicIPClient.Get(ctx, rgName, ipName, nil)
			if err != nil {
				return nil, fmt.Errorf("get public ip: %w", err)
			}
			addr := publicIP.Properties.IPAddress
			if addr != nil {
				ip, err := parseIP(*addr)
				if err != nil {
					return nil, err
				}
				ips = append(ips, *ip)
				logger.Printf("pod vm IP[%d][public]=%s", i, ip.String())
			}
		}
	}

	for i, ipc := range ipcs {
		addr := ipc.Properties.PrivateIPAddress
		if addr == nil {
			return nil, fmt.Errorf("private IP address not found in IP configuration %d", i)
		}
		ip, err := parseIP(*addr)
		if err != nil {
			return nil, err
		}
		ips = append(ips, *ip)
		logger.Printf("pod vm IP[%d][private]=%s", i, ip.String())
	}

	return ips, nil
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

func (p *azureProvider) buildNetworkConfig(nicName string) *armcompute.VirtualMachineNetworkInterfaceConfiguration {
	ipConfig := armcompute.VirtualMachineNetworkInterfaceIPConfiguration{
		Name: to.Ptr("ip-config"),
		Properties: &armcompute.VirtualMachineNetworkInterfaceIPConfigurationProperties{
			Subnet: &armcompute.SubResource{
				ID: to.Ptr(p.serviceConfig.SubnetId),
			},
		},
	}

	if p.serviceConfig.UsePublicIP {
		publicIpConfig := armcompute.VirtualMachinePublicIPAddressConfiguration{
			Name: to.Ptr(nicName),
			Properties: &armcompute.VirtualMachinePublicIPAddressConfigurationProperties{
				DeleteOption: to.Ptr(armcompute.DeleteOptionsDelete),
			},
		}
		ipConfig.Properties.PublicIPAddressConfiguration = &publicIpConfig
	}

	config := armcompute.VirtualMachineNetworkInterfaceConfiguration{
		Name: to.Ptr(nicName),
		Properties: &armcompute.VirtualMachineNetworkInterfaceConfigurationProperties{
			DeleteOption:     to.Ptr(armcompute.DeleteOptionsDelete),
			IPConfigurations: []*armcompute.VirtualMachineNetworkInterfaceIPConfiguration{&ipConfig},
		},
	}

	if p.serviceConfig.SecurityGroupId != "" {
		config.Properties.NetworkSecurityGroup = &armcompute.SubResource{
			ID: to.Ptr(p.serviceConfig.SecurityGroupId),
		}
	}

	return &config
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

	vmParameters, err := p.getVMParameters(instanceSize, diskName, cloudConfigData, sshBytes, instanceName, nicName)
	if err != nil {
		return nil, err
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	vm, err := p.create(ctx, vmParameters)
	if err != nil {
		return nil, fmt.Errorf("Creating instance (%v): %s", vm, err)
	}

	ips, err := p.getIPs(ctx, vm)
	if err != nil {
		logger.Printf("getting IPs for the instance : %v ", err)
		return nil, err
	}

	instance := &provider.Instance{
		ID:   *vm.ID,
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

func (p *azureProvider) getVMParameters(instanceSize, diskName, cloudConfig string, sshBytes []byte, instanceName, nicName string) (*armcompute.VirtualMachine, error) {
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

	networkConfig := p.buildNetworkConfig(nicName)

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
				NetworkAPIVersion:              to.Ptr(armcompute.NetworkAPIVersionTwoThousandTwenty1101),
				NetworkInterfaceConfigurations: []*armcompute.VirtualMachineNetworkInterfaceConfiguration{networkConfig},
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
