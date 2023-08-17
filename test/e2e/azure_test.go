//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"strings"
	"testing"

	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
)

// AzureCloudAssert implements the CloudAssert interface for azure.
type AzureCloudAssert struct{}

var assert = &AzureCloudAssert{}

func TestDeletePodAzure(t *testing.T) {
	t.Parallel()
	doTestDeleteSimplePod(t, assert)
}

func TestCreateSimplePodAzure(t *testing.T) {
	t.Parallel()
	doTestCreateSimplePod(t, assert)
}

func TestCreatePodWithConfigMapAzure(t *testing.T) {
	t.Parallel()
	doTestCreatePodWithConfigMap(t, assert)
}

func TestCreatePodWithSecretAzure(t *testing.T) {
	t.Parallel()
	doTestCreatePodWithSecret(t, assert)
}

func checkVMExistence(resourceGroupName, prefixName string) bool {
	pager := pv.AzureProps.ManagedVmClient.NewListPager(resourceGroupName, nil)

	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			log.Errorf("failed to advance page: %v", err)
			return false
		}

		for _, vm := range page.Value {
			if strings.HasPrefix(*vm.Name, prefixName) {
				// VM found
				return true
			}
		}

	}

	return false
}

func (c AzureCloudAssert) HasPodVM(t *testing.T, id string) {
	pod_vm_prefix := "podvm-" + id
	rg := pv.AzureProps.ResourceGroupName
	if checkVMExistence(rg, pod_vm_prefix) {
		log.Infof("VM found in resource group")
	} else {
		log.Infof("Virtual machine %s not found in resource group %s", id, rg)
		t.Error("PodVM was not created")
	}
}

func (c AzureCloudAssert) getInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM
	return "", nil
}
