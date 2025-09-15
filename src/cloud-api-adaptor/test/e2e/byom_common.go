//go:build byom

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"
	"time"
)

// ByomAssert implements the CloudAssert interface for BYOM.
type ByomAssert struct {
}

func (c ByomAssert) DefaultTimeout() time.Duration {
	return 30 * time.Second
}

func (b ByomAssert) HasPodVM(t *testing.T, id string) {
	// Since BYOM uses pre-created VMs, just log and return
	t.Logf("BYOM: Using pre-created VM for pod %s", id)
}

func (b ByomAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM
	return "", nil
}
