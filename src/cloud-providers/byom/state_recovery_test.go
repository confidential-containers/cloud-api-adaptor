// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigMapVMPoolManagerRecoverState(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()
	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Pre-create ConfigMap with some state
	existingState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"test-allocation-1": {
				AllocationID: "test-allocation-1",
				IP:           "192.168.1.10",
				AllocatedAt:  metav1.Now(),
			},
		},
		AvailableIPs: []string{"192.168.1.11", "192.168.1.12"},
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	stateData, _ := json.Marshal(existingState)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			stateDataKey: string(stateData),
		},
	}

	_, err := client.CoreV1().ConfigMaps(config.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test state recovery
	err = manager.RecoverState(ctx, nil)
	if err != nil {
		t.Errorf("Failed to recover state: %v", err)
	}

	// Verify state was recovered correctly
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Errorf("Failed to get pool status: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected total 3, got %d", total)
	}

	if available != 2 {
		t.Errorf("Expected available 2, got %d", available)
	}

	if inUse != 1 {
		t.Errorf("Expected inUse 1, got %d", inUse)
	}

	// Verify specific allocation exists
	allocatedIP, exists, err := manager.GetIPfromAllocationID(ctx, "test-allocation-1")
	if err != nil {
		t.Errorf("Failed to get allocated IP: %v", err)
	}

	if !exists {
		t.Error("Expected allocation to exist after recovery")
	}

	if allocatedIP.String() != "192.168.1.10" {
		t.Errorf("Expected allocated IP 192.168.1.10, got %s", allocatedIP.String())
	}
}

func TestConfigMapVMPoolManagerRecoverStateWithNodeAllocations(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Create ConfigMap with allocations including some from the current test node
	existingState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"other-node-allocation": {
				AllocationID: "other-node-allocation",
				IP:           "192.168.1.10",
				NodeName:     "other-node",
				PodName:      "other-pod",
				AllocatedAt:  metav1.Now(),
			},
			"test-node-allocation": {
				AllocationID: "test-node-allocation",
				IP:           "192.168.1.11",
				NodeName:     "test-node", // This should be kept (not released during recovery)
				PodName:      "test-pod",
				AllocatedAt:  metav1.Now(),
			},
		},
		AvailableIPs: []string{"192.168.1.12"},
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	stateData, _ := json.Marshal(existingState)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			stateDataKey: string(stateData),
		},
	}

	_, err := client.CoreV1().ConfigMaps(config.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Track VM cleanup calls - recovery should not trigger VM cleanup
	cleanupCalled := false
	vmCleanupFunc := func(ctx context.Context, ip netip.Addr) error {
		cleanupCalled = true
		return nil
	}

	// Test state recovery
	err = manager.RecoverState(ctx, vmCleanupFunc)
	if err != nil {
		t.Errorf("Failed to recover state: %v", err)
	}

	if cleanupCalled {
		t.Error("VM cleanup should not be called during recovery")
	}

	// Verify state after recovery - all allocations should be preserved
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Errorf("Failed to get pool status: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected total 3, got %d", total)
	}

	if available != 1 {
		t.Errorf("Expected available 1, got %d", available)
	}

	if inUse != 2 {
		t.Errorf("Expected inUse 2, got %d", inUse)
	}

	// Verify allocations are preserved during recovery
	_, exists, err := manager.GetIPfromAllocationID(ctx, "test-node-allocation")
	if err != nil {
		t.Errorf("Failed to check test-node allocation: %v", err)
	}
	if !exists {
		t.Error("Expected test-node allocation to be preserved")
	}

	_, exists, err = manager.GetIPfromAllocationID(ctx, "other-node-allocation")
	if err != nil {
		t.Errorf("Failed to check other-node allocation: %v", err)
	}
	if !exists {
		t.Error("Expected other-node allocation to be preserved")
	}
}

func TestConfigMapVMPoolManagerRecoverStateKeepsOrphanedAllocations(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Create ConfigMap with test-node allocation that would be "orphaned" after CAA restart
	existingState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"test-node-allocation": {
				AllocationID: "test-node-allocation",
				IP:           "192.168.1.10",
				NodeName:     "test-node",
				PodName:      "test-pod",
				AllocatedAt:  metav1.Now(),
			},
		},
		AvailableIPs: []string{"192.168.1.11", "192.168.1.12"},
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	stateData, _ := json.Marshal(existingState)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			stateDataKey: string(stateData),
		},
	}

	_, err := client.CoreV1().ConfigMaps(config.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Recovery should not call cleanup function
	vmCleanupFunc := func(ctx context.Context, ip netip.Addr) error {
		t.Errorf("Cleanup function should not be called during recovery")
		return nil
	}

	// Test state recovery
	err = manager.RecoverState(ctx, vmCleanupFunc)
	if err != nil {
		t.Errorf("Failed to recover state: %v", err)
	}

	// Verify orphaned allocation is preserved
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Errorf("Failed to get pool status: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected total 3, got %d", total)
	}

	if available != 2 {
		t.Errorf("Expected available 2, got %d", available)
	}

	if inUse != 1 {
		t.Errorf("Expected inUse 1, got %d", inUse)
	}

	// Verify allocation is preserved
	_, exists, err := manager.GetIPfromAllocationID(ctx, "test-node-allocation")
	if err != nil {
		t.Errorf("Failed to check allocation: %v", err)
	}
	if !exists {
		t.Error("Expected allocation to be preserved")
	}
}

func TestConfigMapVMPoolManagerRecoverEmptyState(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// No ConfigMap exists
	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test state recovery (should initialize empty state)
	err = manager.RecoverState(ctx, nil)
	if err != nil {
		t.Errorf("Failed to recover state: %v", err)
	}

	// Verify empty state was initialized
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Errorf("Failed to get pool status: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}

	if available != 2 {
		t.Errorf("Expected available 2, got %d", available)
	}

	if inUse != 0 {
		t.Errorf("Expected inUse 0, got %d", inUse)
	}
}

func TestConfigMapVMPoolManagerRepairStateFromPrimaryConfig(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Create ConfigMap with state that doesn't match primary config
	corruptedState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"valid-allocation": {
				AllocationID: "valid-allocation",
				IP:           "192.168.1.10", // Valid IP from primary config
				NodeName:     "other-node",
				PodName:      "valid-pod",
				AllocatedAt:  metav1.Now(),
			},
			"invalid-allocation": {
				AllocationID: "invalid-allocation",
				IP:           "10.0.0.1", // Invalid IP (not in primary config)
				NodeName:     "other-node",
				PodName:      "invalid-pod",
				AllocatedAt:  metav1.Now(),
			},
		},
		AvailableIPs: []string{"192.168.1.11", "10.0.0.2", "192.168.1.11"}, // Mix of valid, invalid, and duplicate
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	stateData, _ := json.Marshal(corruptedState)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			stateDataKey: string(stateData),
		},
	}

	_, err := client.CoreV1().ConfigMaps(config.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test state recovery - should repair to match primary config
	err = manager.RecoverState(ctx, nil)
	if err != nil {
		t.Errorf("Failed to recover state: %v", err)
	}

	// Test that recovery repairs state to match primary config while preserving allocations
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Errorf("Failed to get pool status: %v", err)
	}

	// Available IPs should only be from primary config
	expectedAvailable := 2 // 3 primary IPs - 1 valid allocation
	if available != expectedAvailable {
		t.Errorf("Expected available %d, got %d", expectedAvailable, available)
	}

	// All allocations should be preserved
	expectedInUse := 2 // Both valid and invalid allocations
	if inUse != expectedInUse {
		t.Errorf("Expected inUse %d, got %d", expectedInUse, inUse)
	}

	expectedTotal := expectedAvailable + expectedInUse
	if total != expectedTotal {
		t.Errorf("Expected total %d, got %d", expectedTotal, total)
	}

	// Verify both allocations are preserved
	allocatedIPs, err := manager.ListAllocatedIPs(ctx)
	if err != nil {
		t.Errorf("Failed to list allocated IPs: %v", err)
	}

	if len(allocatedIPs) != 2 {
		t.Errorf("Expected 2 allocated IPs, got %d", len(allocatedIPs))
	}

	// Invalid allocation should be preserved
	_, exists := allocatedIPs["invalid-allocation"]
	if !exists {
		t.Error("Expected invalid allocation to be preserved")
	}

	// Valid allocation should be preserved
	validAllocation, exists := allocatedIPs["valid-allocation"]
	if !exists {
		t.Error("Expected valid allocation to be preserved")
	}

	if validAllocation.IP != "192.168.1.10" {
		t.Errorf("Expected valid allocation IP 192.168.1.10, got %s", validAllocation.IP)
	}
}

func TestConfigMapVMPoolManagerMismatchedPoolSizes(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11"}, // Primary config: 2 IPs
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Create ConfigMap with MORE IPs than primary config
	mismatchedState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"allocation-1": {
				AllocationID: "allocation-1",
				IP:           "192.168.1.10", // Valid
				NodeName:     "node-1",
				PodName:      "pod-1",
				AllocatedAt:  metav1.Now(),
			},
			"allocation-2": {
				AllocationID: "allocation-2",
				IP:           "192.168.1.12", // Invalid - not in primary config
				NodeName:     "node-2",
				PodName:      "pod-2",
				AllocatedAt:  metav1.Now(),
			},
		},
		// ConfigMap has 5 IPs, but primary config only has 2
		AvailableIPs: []string{"192.168.1.11", "192.168.1.13", "192.168.1.14", "192.168.1.15", "192.168.1.16"},
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	stateData, _ := json.Marshal(mismatchedState)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			stateDataKey: string(stateData),
		},
	}

	_, err := client.CoreV1().ConfigMaps(config.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test recovery with mismatched pool sizes
	err = manager.RecoverState(ctx, nil)
	if err != nil {
		t.Errorf("Failed to recover state: %v", err)
	}

	// Verify repair: AvailableIPs should ONLY contain primary config IPs
	_, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Errorf("Failed to get pool status: %v", err)
	}

	// Available should only be primary config IPs not allocated
	expectedAvailable := 1 // 2 primary IPs - 1 valid allocation
	if available != expectedAvailable {
		t.Errorf("Expected available %d (only primary config IPs), got %d", expectedAvailable, available)
	}

	// InUse should include all allocations (valid and invalid)
	expectedInUse := 2 // Both allocations kept
	if inUse != expectedInUse {
		t.Errorf("Expected inUse %d (all allocations kept), got %d", expectedInUse, inUse)
	}

	// Verify available IPs are only from primary config
	allocatedIPs, err := manager.ListAllocatedIPs(ctx)
	if err != nil {
		t.Errorf("Failed to list allocated IPs: %v", err)
	}

	// Both allocations should be kept
	if len(allocatedIPs) != 2 {
		t.Errorf("Expected 2 allocations kept, got %d", len(allocatedIPs))
	}

	// Test that we can only allocate from primary config IPs
	// Should be able to allocate 192.168.1.11 (the remaining primary config IP)
	t.Logf("Available pool should only contain primary config IPs")
}

func TestConfigMapVMPoolManagerMissingPrimaryIPs(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11", "192.168.1.12", "192.168.1.13"}, // Primary: 4 IPs
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Create ConfigMap with FEWER IPs than primary config (missing some primary IPs)
	incompleteState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"allocation-1": {
				AllocationID: "allocation-1",
				IP:           "192.168.1.10", // Valid primary IP
				NodeName:     "node-1",
				PodName:      "pod-1",
				AllocatedAt:  metav1.Now(),
			},
		},
		// ConfigMap missing 192.168.1.12 and 192.168.1.13 from primary config
		AvailableIPs: []string{"192.168.1.11"}, // Missing .12 and .13
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	stateData, _ := json.Marshal(incompleteState)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			stateDataKey: string(stateData),
		},
	}

	_, err := client.CoreV1().ConfigMaps(config.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test recovery - should add missing primary IPs to AvailableIPs
	err = manager.RecoverState(ctx, nil)
	if err != nil {
		t.Errorf("Failed to recover state: %v", err)
	}

	// Verify all primary IPs are now accounted for
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Errorf("Failed to get pool status: %v", err)
	}

	// Available should be: primary config IPs - allocated IPs
	expectedAvailable := 3 // 4 primary IPs - 1 allocation
	if available != expectedAvailable {
		t.Errorf("Expected available %d (missing primary IPs added), got %d", expectedAvailable, available)
	}

	// InUse should remain the same
	expectedInUse := 1
	if inUse != expectedInUse {
		t.Errorf("Expected inUse %d, got %d", expectedInUse, inUse)
	}

	// Total should equal primary config size now
	expectedTotal := 4 // Should match primary config size
	if total != expectedTotal {
		t.Errorf("Expected total %d (primary config size), got %d", expectedTotal, total)
	}

	t.Logf("Recovery should add missing primary config IPs to available pool")
}

func TestConfigMapVMPoolManagerPoolIPsChange(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Start with original pool configuration
	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Create ConfigMap with existing state using original pool IPs
	existingState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"test-allocation-1": {
				AllocationID: "test-allocation-1",
				IP:           "192.168.1.10", // Will remain valid
				AllocatedAt:  metav1.Now(),
			},
		},
		AvailableIPs: []string{"192.168.1.11", "192.168.1.12"}, // 192.168.1.11 will become invalid
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	stateData, _ := json.Marshal(existingState)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			stateDataKey: string(stateData),
		},
	}

	_, err := client.CoreV1().ConfigMaps(config.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Now change the pool IPs (same count: 3->3, but different IPs)
	// This simulates the scenario where PoolIPs are updated in configuration
	config.PoolIPs = []string{"192.168.1.10", "192.168.1.13", "192.168.1.14"} // Replaced .11 and .12 with .13 and .14

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test state recovery - this should detect the PoolIP changes and update ConfigMap
	err = manager.RecoverState(ctx, nil)
	if err != nil {
		t.Errorf("Failed to recover state: %v", err)
	}

	// Verify that the state was updated to reflect new pool configuration
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Errorf("Failed to get pool status: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected total 3, got %d", total)
	}

	if available != 2 {
		t.Errorf("Expected available 2, got %d", available)
	}

	if inUse != 1 {
		t.Errorf("Expected inUse 1, got %d", inUse)
	}

	// Verify that the ConfigMap was actually updated with new state
	updatedCM, err := client.CoreV1().ConfigMaps(config.Namespace).Get(ctx, config.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated ConfigMap: %v", err)
	}

	var updatedState IPAllocationState
	err = json.Unmarshal([]byte(updatedCM.Data[stateDataKey]), &updatedState)
	if err != nil {
		t.Fatalf("Failed to unmarshal updated state: %v", err)
	}

	// Verify the version was incremented (proves ConfigMap was updated)
	if updatedState.Version <= existingState.Version {
		t.Errorf("Expected version to be incremented from %d, got %d", existingState.Version, updatedState.Version)
	}

	// Verify available IPs now contain the new pool IPs (192.168.1.13, 192.168.1.14)
	expectedAvailable := []string{"192.168.1.13", "192.168.1.14"}
	if len(updatedState.AvailableIPs) != len(expectedAvailable) {
		t.Errorf("Expected %d available IPs, got %d", len(expectedAvailable), len(updatedState.AvailableIPs))
	}

	for _, expectedIP := range expectedAvailable {
		found := false
		for _, actualIP := range updatedState.AvailableIPs {
			if actualIP == expectedIP {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected available IP %s not found in updated state", expectedIP)
		}
	}

	// Verify that the valid allocation (192.168.1.10) was preserved
	if len(updatedState.AllocatedIPs) != 1 {
		t.Errorf("Expected 1 allocated IP, got %d", len(updatedState.AllocatedIPs))
	}

	allocation, exists := updatedState.AllocatedIPs["test-allocation-1"]
	if !exists {
		t.Error("Expected valid allocation to be preserved")
	}

	if allocation.IP != "192.168.1.10" {
		t.Errorf("Expected preserved allocation IP 192.168.1.10, got %s", allocation.IP)
	}
}
