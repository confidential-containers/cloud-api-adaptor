// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CloudDiskVerifier checks the state of cloud-managed disks via provider APIs.
type CloudDiskVerifier interface {
	DiskExists(ctx context.Context, diskID string) (bool, error)
	DiskState(ctx context.Context, diskID string) (string, error)
}

var cloudDiskVerifierFactories = map[string]func() (CloudDiskVerifier, error){}

func registerCloudDiskVerifier(provider string, factory func() (CloudDiskVerifier, error)) {
	cloudDiskVerifierFactories[provider] = factory
}

func newCloudDiskVerifier(provider string) (CloudDiskVerifier, error) {
	factory, ok := cloudDiskVerifierFactories[strings.ToLower(provider)]
	if !ok {
		return nil, fmt.Errorf("unsupported cloud provider for disk verification: %s (is the correct build tag set?)", provider)
	}
	return factory()
}

// waitForDiskDetached polls until the disk reaches a detached state
// ("available" on AWS, "Unattached" on Azure) or times out.
// Returns early if the disk was already deleted (no need to wait for detach).
func waitForDiskDetached(ctx context.Context, verifier CloudDiskVerifier, diskID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var lastState string
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return lastState, fmt.Errorf("context cancelled while waiting for disk detach: %w", err)
		}
		state, err := verifier.DiskState(ctx, diskID)
		if err != nil {
			return "", fmt.Errorf("checking disk state: %w", err)
		}
		lastState = state
		lower := strings.ToLower(state)
		if lower == "available" || lower == "unattached" {
			return state, nil
		}
		if lower == "deleted" {
			return "deleted (disk already removed)", nil
		}
		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return lastState, fmt.Errorf("context cancelled while waiting for disk detach: %w", ctx.Err())
		}
	}
	return lastState, fmt.Errorf("disk %s did not detach within %v (last state: %s)", diskID, timeout, lastState)
}

// waitForDiskDeleted polls until the disk no longer exists or times out.
func waitForDiskDeleted(ctx context.Context, verifier CloudDiskVerifier, diskID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled while waiting for disk deletion: %w", err)
		}
		exists, err := verifier.DiskExists(ctx, diskID)
		if err != nil {
			return fmt.Errorf("checking disk existence: %w", err)
		}
		if !exists {
			return nil
		}
		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for disk deletion: %w", ctx.Err())
		}
	}
	return fmt.Errorf("disk %s still exists after %v", diskID, timeout)
}
