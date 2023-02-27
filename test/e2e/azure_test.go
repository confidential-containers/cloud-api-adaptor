//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

func initResourceGroup() (*resources.Group, error) {
	/*
		Set following environment variables to verify identity.
		1. AZURE_SUBSCRIPTION_ID - Subscription in which resources will be created
		2. RESOURCE_GROUP - Resource group( For now it will be pre created group)
		3. AZURE_CLIENT_ID - azure client id
		4. AZURE_CLIENT_SECRET - azure client secret
		5. AZURE_TENANT_ID - azure tenant id

	*/
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if len(subscriptionID) <= 0 {
		return nil, errors.New("AZURE_SUBSCRIPTION_ID was not set")
	}

	resourceGroup := os.Getenv("RESOURCE_GROUP")
	if len(resourceGroup) <= 0 {
		return nil, errors.New("RESOURCE_GROUP was not set")
	}

	if (len(os.Getenv("AZURE_CLIENT_ID")) <= 0) || (len(os.Getenv("AZURE_CLIENT_SECRET")) <= 0) || (len(os.Getenv("AZURE_TENANT_ID")) <= 0) {
		return nil, errors.New("Please set all the env variables mentioned")
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, err
	}

	groupsClient := resources.NewGroupsClient(subscriptionID)
	groupsClient.Authorizer = authorizer

	group, err := groupsClient.Get(context.Background(), resourceGroup)
	if err != nil {
		return nil, err
	}

	return &group, nil
}

func TestCreateSimplePodAzure(t *testing.T) {
	group, err := initResourceGroup()
	if err != nil {
		t.Fatal(err)
	}
	assert := AzureCloudAssert{
		group,
	}
	doTestCreateSimplePod(t, assert)
}

// AzureCloudAssert implements the CloudAssert interface for azure.
type AzureCloudAssert struct {
	group *resources.Group
}

func (c AzureCloudAssert) HasPodVM(t *testing.T, id string) {
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if len(subscriptionID) <= 0 {
		t.Fatal(errors.New("AZURE_SUBSCRIPTION_ID was not set"))
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("PodVM name: ", id)

	vmClient := compute.NewVirtualMachinesClient(subscriptionID)
	vmClient.Authorizer = authorizer

	vm, err := vmClient.Get(context.Background(), *c.group.Name, id, "")
	if err != nil {
		if vmNotFound, ok := err.(autorest.DetailedError); ok && vmNotFound.StatusCode == http.StatusNotFound {
			fmt.Printf("Virtual machine '%s' not found in resource group '\n", id)
		} else {
			t.Fatal(err)
		}
	}

	fmt.Printf("VM name: %s\n", *vm.Name)
	fmt.Printf("VM location: %s\n", *vm.Location)
}
