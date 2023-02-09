// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/azure] ", log.LstdFlags|log.Lmsgprefix)
var errNotReady = errors.New("address not ready")

const (
	maxInstanceNameLen = 63
	DefaultUserName    = "ubuntu"
)

type azureProvider struct {
	azureClient   azcore.TokenCredential
	serviceConfig *Config
}

func NewProvider(config *Config) (cloud.Provider, error) {

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

	return provider, nil
}

func getIPs(nic *armnetwork.Interface) ([]net.IP, error) {
	var podNodeIPs []net.IP

	for i, ipc := range nic.Properties.IPConfigurations {
		addr := ipc.Properties.PrivateIPAddress

		if addr == nil || *addr == "" || *addr == "0.0.0.0" {
			return nil, errNotReady
		}

		ip := net.ParseIP(*addr)
		if ip == nil {
			return nil, fmt.Errorf("parsing pod node IP %q", *addr)
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

func (p *azureProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator) (*cloud.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))

	diskName := fmt.Sprintf("%s-disk", instanceName)
	nicName := fmt.Sprintf("%s-net", instanceName)

	// Get NIC using subnet and allow ports on the ssh group
	vmNIC, err := p.createNetworkInterface(ctx, nicName)
	if err != nil {
		err = fmt.Errorf("creating VM network interface: %w", err)
		logger.Printf("%v", err)
		return nil, err
	}

	// require ssh key for authentication on linux
	sshPublicKeyPath := os.ExpandEnv(p.serviceConfig.SSHKeyPath)
	var sshBytes []byte
	if _, err := os.Stat(sshPublicKeyPath); err == nil {
		sshBytes, err = ioutil.ReadFile(sshPublicKeyPath)
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

	vmParameters := armcompute.VirtualMachine{
		Location: to.Ptr(p.serviceConfig.Region),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(p.serviceConfig.Size)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					ID: to.Ptr(p.serviceConfig.ImageId),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr(diskName),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					Caching:      to.Ptr(armcompute.CachingTypesReadWrite),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				AdminUsername: to.Ptr(DefaultUserName),
				ComputerName:  to.Ptr(instanceName),
				CustomData:    to.Ptr(userDataEnc),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					//TBD: replace with a suitable mechanism to use precreated SSH key
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{{
							Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", DefaultUserName)),
							KeyData: to.Ptr(string(sshBytes)),
						}},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{ID: vmNIC.ID},
				},
			},
		},
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	result, err := p.create(context.TODO(), &vmParameters)
	if err != nil {
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

	pollerResponse, err := vmClient.BeginDelete(ctx, p.serviceConfig.ResourceGroupName, instanceID, nil)
	if err != nil {
		return fmt.Errorf("beginning VM deletion: %w", err)
	}

	if _, err = pollerResponse.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("waiting for the VM deletion: %w", err)
	}

	logger.Printf("deleted VM successfully: %s", instanceID)

	return nil
}

func (p *azureProvider) Teardown() error {
	return nil
}
