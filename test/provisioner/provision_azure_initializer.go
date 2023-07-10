// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	log "github.com/sirupsen/logrus"
)

type AzureProperties struct {
	ResourceGroup     *armresources.ResourceGroup
	CloudProvider     string
	SubscriptionID    string
	ClientID          string
	ClientSecret      string
	TenantID          string
	ResourceGroupName string
	ClusterName       string
	Location          string
	SshPrivateKey     string
	SubnetName        string
	VnetName          string
	SubnetID          string
	ImageID           string
	SshUserName       string
	IsAzCliAuth       bool
	IsCIManaged       bool

	InstanceSize string
	NodeName     string
	OsType       string

	ResourceGroupClient *armresources.ResourceGroupsClient
	ManagedVnetClient   *armnetwork.VirtualNetworksClient
	ManagedSubnetClient *armnetwork.SubnetsClient
	ManagedAksClient    *armcontainerservice.ManagedClustersClient
	ManagedVmClient     *armcompute.VirtualMachinesClient
}

var AzureProps = &AzureProperties{}

func initAzureProperties(properties map[string]string) error {
	log.Trace("initazureProperties()")
	AzureProps = &AzureProperties{
		SubscriptionID:    properties["AZURE_SUBSCRIPTION_ID"],
		ClientID:          properties["AZURE_CLIENT_ID"],
		ClientSecret:      properties["AZURE_CLIENT_SECRET"],
		TenantID:          properties["AZURE_TENANT_ID"],
		ResourceGroupName: properties["RESOURCE_GROUP_NAME"],
		ClusterName:       properties["CLUSTER_NAME"],
		Location:          properties["LOCATION"],
		SshPrivateKey:     properties["SSH_KEY_ID"],
		CloudProvider:     properties["CLOUD_PROVIDER"],
		ImageID:           properties["AZURE_IMAGE_ID"],
		SubnetID:          properties["AZURE_SUBNET_ID"],
		SshUserName:       properties["SSH_USERNAME"],
	}

	CIManagedStr := properties["IS_CI_MANAGED_CLUSTER"]
	AzureProps.IsCIManaged = false
	if strings.EqualFold(CIManagedStr, "yes") || strings.EqualFold(CIManagedStr, "true") {
		AzureProps.IsCIManaged = true
	}

	CliAuthStr := properties["AZURE_CLI_AUTH"]
	AzureProps.IsAzCliAuth = false
	if strings.EqualFold(CliAuthStr, "yes") || strings.EqualFold(CliAuthStr, "true") {
		AzureProps.IsAzCliAuth = true
	}

	AzureProps.VnetName = AzureProps.ClusterName + "_vnet"
	AzureProps.SubnetName = AzureProps.ClusterName + "_subnet"
	AzureProps.InstanceSize = "Standard_D2as_v5"
	AzureProps.NodeName = "caaaks"
	AzureProps.OsType = "Ubuntu"

	if AzureProps.SubscriptionID == "" {
		return errors.New("AZURE_SUBSCRIPTION_ID was not set.")
	}
	if AzureProps.ClientID == "" && !AzureProps.IsAzCliAuth {
		return errors.New("AZURE_CLIENT_ID was not set.")
	}
	if AzureProps.ClientSecret == "" && !AzureProps.IsAzCliAuth {
		return errors.New("AZURE_CLIENT_SECRET was not set")
	}
	if AzureProps.TenantID == "" {
		return errors.New("AZURE_TENANT_ID was not set")
	}
	if AzureProps.Location == "" {
		return errors.New("LOCATION was not set.")
	}
	if AzureProps.CloudProvider == "" {
		return errors.New("CLOUD_PROVIDER was not set.")
	}
	if AzureProps.SshPrivateKey == "" {
		return errors.New("SSH_KEY_ID was not set.")
	}
	if AzureProps.ImageID == "" {
		return errors.New("AZURE_IMAGE_ID was not set.")
	}
	if AzureProps.ClusterName == "" {
		AzureProps.ClusterName = "e2e_test_cluster"
	}
	if AzureProps.ResourceGroupName == "" {
		AzureProps.ResourceGroupName = AzureProps.ClusterName + "_rg"
	}

	err := initManagedClients()
	if err != nil {
		return fmt.Errorf("Failed initialising managed clients:%w", err)
	}

	return nil
}

func initManagedClients() error {
	log.Trace("initManagedClients()")
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return err
	}

	resourcesClientFactory, err := armresources.NewClientFactory(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("Failed creating resource client factory:%w", err)
	}
	resourceGroupClient := resourcesClientFactory.NewResourceGroupsClient()

	networkClientFactory, err := armnetwork.NewClientFactory(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("Failed creating network client factory:%w", err)
	}
	virtualNetworksClient := networkClientFactory.NewVirtualNetworksClient()
	subnetsClient := networkClientFactory.NewSubnetsClient()

	computeClientFactory, err := armcompute.NewClientFactory(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("Failed creating compute client factory:%w", err)
	}

	virtualMachinesClient := computeClientFactory.NewVirtualMachinesClient()

	containerserviceClientFactory, err := armcontainerservice.NewClientFactory(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("Failed creating container service client factory:%w", err)
	}
	managedClustersClient := containerserviceClientFactory.NewManagedClustersClient()

	AzureProps.ResourceGroupClient = resourceGroupClient
	AzureProps.ManagedAksClient = managedClustersClient
	AzureProps.ManagedVmClient = virtualMachinesClient
	AzureProps.ManagedSubnetClient = subnetsClient
	AzureProps.ManagedVnetClient = virtualNetworksClient
	return nil
}
