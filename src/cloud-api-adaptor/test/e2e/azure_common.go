// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/azure"
	log "github.com/sirupsen/logrus"
)

// AzureCloudAssert implements the CloudAssert interface for azure.
type AzureCloudAssert struct{}

var assert = &AzureCloudAssert{}

// findVM is a helper function to find a VM by its prefix name in a resource group.
func findVM(resourceGroupName, prefixName string) (*armcompute.VirtualMachine, error) {
	pager := pv.AzureProps.ManagedVMClient.NewListPager(resourceGroupName, nil)

	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			log.Errorf("failed to advance page: %v", err)
			return nil, err
		}

		for _, vm := range page.Value {
			if strings.HasPrefix(*vm.Name, prefixName) {
				// VM found
				return vm, nil
			}
		}
	}

	return nil, nil
}

func checkVMExistence(resourceGroupName, prefixName string) bool {
	vm, err := findVM(resourceGroupName, prefixName)
	if err != nil {
		return false
	}
	return vm != nil
}

func (c AzureCloudAssert) DefaultTimeout() time.Duration {
	return 2 * time.Minute
}

func (c AzureCloudAssert) HasPodVM(t *testing.T, id string) {
	podVMPrefix := "podvm-" + id
	rg := pv.AzureProps.ResourceGroupName
	if checkVMExistence(rg, podVMPrefix) {
		t.Logf("VM %s found in resource group", id)
	} else {
		t.Logf("Virtual machine %s not found in resource group %s", id, rg)
		t.Error("PodVM was not created")
	}
}

func (c AzureCloudAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM

	prefixName := "podvm-" + podName
	vm, err := findVM(pv.AzureProps.ResourceGroupName, prefixName)
	if err != nil {
		t.Logf("Virtual machine %s not found in resource group %s", podName, pv.AzureProps.ResourceGroupName)
		return "", err
	}

	// VM found
	// Extract the VM size
	if vm != nil && vm.Properties != nil && vm.Properties.HardwareProfile != nil {
		t.Logf("The VM size for VM '%s' is: %s\n", podName, *vm.Properties.HardwareProfile.VMSize)
		return string(*vm.Properties.HardwareProfile.VMSize), nil
	} else {
		log.Errorf("Failed to get VM size for VM '%s'", podName)
		return "", nil
	}

}
