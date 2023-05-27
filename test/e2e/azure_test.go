//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	//"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
)

func TestCreateSimplePodAzure(t *testing.T) {
	assert := AzureCloudAssert{
		group: pv.AzureProps.ResourceGroup,
	}
	doTestCreateSimplePod(t, assert)
}

func TestCreatePodWithConfigMapAzure(t *testing.T) {
	assert := AzureCloudAssert{
		group: pv.AzureProps.ResourceGroup,
	}
	doTestCreatePodWithConfigMap(t, assert)
}

func TestCreatePodWithSecretAzure(t *testing.T) {
	assert := AzureCloudAssert{
		group: pv.AzureProps.ResourceGroup,
	}
	doTestCreatePodWithSecret(t, assert)
}

// AzureCloudAssert implements the CloudAssert interface for azure.
type AzureCloudAssert struct {
	group *armresources.ResourceGroup
}

func checkVMExistence(resourceGroupName, prefixName string) bool {
	//vmList, err := pv.AzureProps.ManagedVmClient.List(resourceGroupName)
	/*if err != nil {
		return false
	}

	for _, vm := range vmList.Value() {
		if strings.HasPrefix(*vm.Name, prefixName) {
			// VM found
			return true
		}
	}
	*/
	return false
}

func (c AzureCloudAssert) HasPodVM(t *testing.T, id string) {
	pod_vm_prefix := "podvm-" + id
	if checkVMExistence(*c.group.Name, pod_vm_prefix) {
		log.Infof("VM found in resource group")
	} else {
		log.Infof("Virtual machine %s not found in resource group ", id)
		t.Error("PodVM was not created")
	}
}
