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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	log "github.com/sirupsen/logrus"
)

type AzureProperties struct {
	ResourceGroup       *armresources.ResourceGroup
	SubscriptionID      string
	ClientID            string
	ClientSecret        string
	TenantID            string
	ResourceGroupName   string
	ClusterName         string
	Location            string
	SshPrivateKey       string
	SubnetName          string
	VnetName            string
	SubnetID            string
	ImageID             string
	SshUserName         string
	ManagedIdentityName string
	IsAzCliAuth         bool
	IsCIManaged         bool

	InstanceSize string
	NodeName     string
	OsType       string

	ResourceGroupClient                *armresources.ResourceGroupsClient
	ManagedVnetClient                  *armnetwork.VirtualNetworksClient
	ManagedSubnetClient                *armnetwork.SubnetsClient
	ManagedAksClient                   *armcontainerservice.ManagedClustersClient
	ManagedVmClient                    *armcompute.VirtualMachinesClient
	FederatedIdentityCredentialsClient *armmsi.FederatedIdentityCredentialsClient
	federatedIdentityCredentialName    string
}

var AzureProps = &AzureProperties{}

func initAzureProperties(properties map[string]string) error {
	log.Trace("initazureProperties()")
	AzureProps = &AzureProperties{
		SubscriptionID:      properties["AZURE_SUBSCRIPTION_ID"],
		ClientID:            properties["AZURE_CLIENT_ID"],
		ClientSecret:        properties["AZURE_CLIENT_SECRET"],
		TenantID:            properties["AZURE_TENANT_ID"],
		ResourceGroupName:   properties["RESOURCE_GROUP_NAME"],
		ClusterName:         properties["CLUSTER_NAME"],
		Location:            properties["LOCATION"],
		SshPrivateKey:       properties["SSH_KEY_ID"],
		ImageID:             properties["AZURE_IMAGE_ID"],
		SubnetID:            properties["AZURE_SUBNET_ID"],
		SshUserName:         properties["SSH_USERNAME"],
		ManagedIdentityName: properties["MANAGED_IDENTITY_NAME"],
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

	// TODO: Right now AZURE_CLIENT_ID can be used by the provisioner
	// application and the same value is passed on to the CAA app inside the
	// daemonset. Figure out a way to separate these two.
	if AzureProps.ClientID == "" {
		return errors.New("AZURE_CLIENT_ID was not set.")
	}
	if AzureProps.ClientSecret == "" && !AzureProps.IsAzCliAuth {
		return errors.New("AZURE_CLIENT_SECRET was not set")
	}
	if AzureProps.Location == "" {
		return errors.New("LOCATION was not set.")
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
		return fmt.Errorf("initializing managed clients: %w", err)
	}

	AzureProps.federatedIdentityCredentialName = fmt.Sprintf("%sFederatedIdentityCredential", AzureProps.ClusterName)

	return nil
}

func initManagedClients() error {
	log.Trace("initManagedClients()")

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("creating azure credential: %w", err)
	}

	AzureProps.ResourceGroupClient, err = armresources.NewResourceGroupsClient(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("creating resource group client: %w", err)
	}

	// Use a client factory when creating multiple of these clients.
	networkClientFactory, err := armnetwork.NewClientFactory(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("creating network client factory: %w", err)
	}
	AzureProps.ManagedVnetClient = networkClientFactory.NewVirtualNetworksClient()
	AzureProps.ManagedSubnetClient = networkClientFactory.NewSubnetsClient()

	AzureProps.ManagedVmClient, err = armcompute.NewVirtualMachinesClient(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("creating virtual machine client: %w", err)
	}

	AzureProps.ManagedAksClient, err = armcontainerservice.NewManagedClustersClient(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("creating managed cluster client: %w", err)
	}

	AzureProps.FederatedIdentityCredentialsClient, err = armmsi.NewFederatedIdentityCredentialsClient(AzureProps.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("creating federated identity credentials client: %w", err)
	}

	return nil
}
