// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/containerservice/mgmt/containerservice"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

type AzureProperties struct {
	ResourceGroup     *resources.Group
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

	InstanceSize string
	NodeName     string
	OsType       string

	ResourceGroupClient *resources.GroupsClient
	ManagedVnetClient   *network.VirtualNetworksClient
	ManagedSubnetClient *network.SubnetsClient
	ManagedAksClient    *containerservice.ManagedClustersClient
	ManagedVmClient     *compute.VirtualMachinesClient
}

var AzureProps = &AzureProperties{}

func initAzureProperties(properties map[string]string) error {
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
	}

	if len(AzureProps.SubscriptionID) <= 0 {
		return errors.New("AZURE_SUBSCRIPTION_ID was not set.")
	}
	if len(AzureProps.ClientID) <= 0 {
		return errors.New("AZURE_CLIENT_ID was not set.")
	}
	if len(AzureProps.ClientSecret) <= 0 {
		return errors.New("AZURE_CLIENT_SECRET was not set")
	}
	if len(AzureProps.TenantID) <= 0 {
		return errors.New("AZURE_TENANT_ID was not set")
	}
	if len(AzureProps.Location) <= 0 {
		return errors.New("LOCATION was not set.")
	}
	if len(AzureProps.CloudProvider) <= 0 {
		return errors.New("CLOUD_PROVIDER was not set.")
	}
	if len(AzureProps.SshPrivateKey) <= 0 {
		return errors.New("SSH_KEY_ID was not set.")
	}
	if len(AzureProps.ClusterName) <= 0 {
		AzureProps.ClusterName = "e2e_test_cluster"
	}
	if len(AzureProps.ResourceGroupName) <= 0 {
		AzureProps.ResourceGroupName = AzureProps.ClusterName + "_rg"
	}

	AzureProps.VnetName = AzureProps.ClusterName + "_vnet"
	AzureProps.SubnetName = AzureProps.ClusterName + "_subnet"
	AzureProps.InstanceSize = "Standard_D2as_v5"
	AzureProps.NodeName = "caaaks"
	AzureProps.OsType = "Ubuntu"

	err := initManagedClients()
	if err != nil {
		return fmt.Errorf("Initialising managed clients:%w", err)
	}

	return nil
}

func initManagedClients() error {
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return err
	}

	groupsClient := resources.NewGroupsClient(AzureProps.SubscriptionID)
	groupsClient.Authorizer = authorizer

	vnetClient := network.NewVirtualNetworksClient(AzureProps.SubscriptionID)
	vnetClient.Authorizer = authorizer

	aksClient := containerservice.NewManagedClustersClient(AzureProps.SubscriptionID)
	aksClient.Authorizer = authorizer

	subnetClient := network.NewSubnetsClient(AzureProps.SubscriptionID)
	subnetClient.Authorizer = authorizer

	vmClient := compute.NewVirtualMachinesClient(AzureProps.SubscriptionID)
	vmClient.Authorizer = authorizer

	AzureProps.ResourceGroupClient = &groupsClient
	AzureProps.ManagedVnetClient = &vnetClient
	AzureProps.ManagedSubnetClient = &subnetClient
	AzureProps.ManagedAksClient = &aksClient
	AzureProps.ManagedVmClient = &vmClient

	return nil
}
