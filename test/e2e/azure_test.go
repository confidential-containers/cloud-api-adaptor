//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest"
	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
)

func TestCreateSimplePodAzure(t *testing.T) {
	assert := AzureCloudAssert{
		group: pv.AzureProps.ResourceGroup,
	}
	doTestCreateSimplePod(t, assert)
}

// AzureCloudAssert implements the CloudAssert interface for azure.
type AzureCloudAssert struct {
	group *resources.Group
}

func (c AzureCloudAssert) HasPodVM(t *testing.T, id string) {
	vm, err := pv.AzureProps.ManagedVmClient.Get(context.Background(), *c.group.Name, id, "")
	if err != nil {
		if vmNotFound, ok := err.(autorest.DetailedError); ok && vmNotFound.StatusCode == http.StatusNotFound {
			log.Infof("Virtual machine %s not found in resource group ", id)
		} else {
			t.Fatal(err)
		}
	}

	log.Infof("VM name: %s", *vm.Name)
	log.Infof("VM location: %s", *vm.Location)
}
