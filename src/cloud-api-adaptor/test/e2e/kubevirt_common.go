// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"
	"time"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/kubevirt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeVirtCloudAssert struct{}

// findKubeVirtVM finds a VirtualMachine by its name in all namespaces.
func findKubeVirtVM(t *testing.T, podvmName string) bool {
	client := pv.KubeVirtProvs.KubevirtClient()
	if client == nil {
		t.Logf("kubevirt client is not initialized")
		return false
	}
	vmlist, err := client.VirtualMachine(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Logf("failed to list VMs: %s", err)
		return false
	}
	for i := range vmlist.Items {
		if vmlist.Items[i].ObjectMeta.Name == podvmName {
			return true
		}
	}
	t.Logf("VM with name %s not found", podvmName)
	return false
}

// NewKubeVirtCloudAssert creates a new instance of KubeVirtCloudAssert.
func NewKubeVirtCloudAssert() KubeVirtCloudAssert {
	return KubeVirtCloudAssert{}
}

// DefaultTimeout returns the default timeout for assertions in KubeVirt tests.
func (c KubeVirtCloudAssert) DefaultTimeout() time.Duration {
	return 2 * time.Minute
}

// HasPodVM checks if a VirtualMachine with the given name exists.
func (c KubeVirtCloudAssert) HasPodVM(t *testing.T, podvmName string) {
	server := findKubeVirtVM(t, podvmName)
	if server == false {
		t.Logf("Virtual machine %s not found", podvmName)
		t.Error("PodVM was not created")
	}
}

func (c KubeVirtCloudAssert) GetInstanceType(t *testing.T, podvmName string) (string, error) {
	// Get Instance Type of PodVM
	return "", nil
}
