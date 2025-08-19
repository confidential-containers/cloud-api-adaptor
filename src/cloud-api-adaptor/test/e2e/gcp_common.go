// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/gcp"
	log "github.com/sirupsen/logrus"
)

// GCPCloudAssert implements the CloudAssert interface for gcp.
type GCPCloudAssert struct{}

func NewGCPAssert() GCPCloudAssert {
	return GCPCloudAssert{}
}

func (c GCPCloudAssert) DefaultTimeout() time.Duration {
	return 2 * time.Minute
}

func gcpFindVM(prefixName string) (*computepb.Instance, error) {
	ctx := context.Background()
	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewInstancesRESTClient: %w", err)
	}
	defer instancesClient.Close()

	filter := fmt.Sprintf("name eq ^%s.*", prefixName)
	req := &computepb.ListInstancesRequest{
		Project: pv.GCPProps.GkeCluster.ProjectID,
		Zone:    pv.GCPProps.GkeCluster.Zone,
		Filter:  &filter,
	}

	it := instancesClient.List(ctx, req)
	instance, err := it.Next()
	if err != nil {
		return nil, err
	}

	log.Infof("Instance found %s %s", instance.GetName(), instance.GetMachineType())
	return instance, nil
}

func (c GCPCloudAssert) HasPodVM(t *testing.T, id string) {
	podVMPrefix := "podvm-" + id
	vm, err := gcpFindVM(podVMPrefix)
	if vm != nil {
		t.Logf("Vitural machine %s found.", id)
	} else {
		t.Logf("Virtual machine %s not found: %s", id, err)
		t.Error("PodVM was not created")
	}
}

func (c GCPCloudAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM

	prefixName := "podvm-" + podName
	vm, err := gcpFindVM(prefixName)
	if err != nil {
		t.Logf("Virtual machine %s not found: %s", podName, err)
		return "", err
	}

	return vm.GetMachineType(), nil
}
