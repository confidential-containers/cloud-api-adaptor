// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/ecloud"
	ecloudprovider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/ecloud"
)

// EcloudAssert implements the CloudAssert interface for ecloud.
type EcloudAssert struct{}

// NewEcloudAssert creates a new EcloudAssert.
func NewEcloudAssert() EcloudAssert {
	return EcloudAssert{}
}

// DefaultTimeout returns the default timeout for ecloud operations.
func (e EcloudAssert) DefaultTimeout() time.Duration {
	return 2 * time.Minute
}

// HasPodVM asserts that a PodVM with the given name exists on ecloud.
func (e EcloudAssert) HasPodVM(t *testing.T, podvmName string) {
	cfg := &ecloudprovider.Config{
		AccessKeyID: ecloud.EcloudProps.AccessKeyID,
		SecretKey:   ecloud.EcloudProps.SecretKey,
		Region:      ecloud.EcloudProps.Region,
		ZoneID:      ecloud.EcloudProps.ZoneID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.DefaultTimeout())
	defer cancel()

	exists, err := ecloudprovider.InstanceExistsByName(ctx, cfg, podvmName)
	if err != nil {
		t.Errorf("failed to check PodVM %s existence: %v", podvmName, err)
		return
	}
	if !exists {
		t.Errorf("PodVM %s was not created", podvmName)
	}
}

// GetInstanceType returns the instance type of the PodVM for the given pod.
// Currently returns the configured instance type as ecloud API does not
// expose a per-pod query by name pattern in the test flow.
func (e EcloudAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	return ecloud.EcloudProps.InstanceType, nil
}
