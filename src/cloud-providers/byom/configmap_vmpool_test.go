// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// setupTestEnvironment sets up environment for tests
func setupTestEnvironment(t *testing.T) func() {
	// Set test node name
	originalNodeName := os.Getenv("NODE_NAME")
	os.Setenv("NODE_NAME", "test-node")

	// Return cleanup function
	return func() {
		if originalNodeName != "" {
			os.Setenv("NODE_NAME", originalNodeName)
		} else {
			os.Unsetenv("NODE_NAME")
		}
	}
}

func TestNewConfigMapVMPoolManager(t *testing.T) {
	config := &GlobalVMPoolConfig{
		Namespace:     "test-namespace",
		ConfigMapName: "test-configmap",
		PoolIPs:       []string{"192.168.1.10", "192.168.1.11"},
	}

	client := fake.NewSimpleClientset()

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Errorf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	if manager == nil {
		t.Error("Expected manager to be non-nil")
	}
}

func TestNewConfigMapVMPoolManagerValidation(t *testing.T) {
	config := &GlobalVMPoolConfig{
		Namespace:     "test-namespace",
		ConfigMapName: "test-configmap",
		PoolIPs:       []string{"192.168.1.10", "invalid-ip"},
	}

	client := fake.NewSimpleClientset()

	_, err := NewConfigMapVMPoolManager(client, config)
	if err == nil {
		t.Error("Expected error for invalid IP address")
	}
}

func TestNewConfigMapVMPoolManagerNilClient(t *testing.T) {
	config := &GlobalVMPoolConfig{
		PoolIPs: []string{"192.168.1.10"},
	}

	_, err := NewConfigMapVMPoolManager(nil, config)
	if err == nil {
		t.Error("Expected error for nil client")
	}
}

func TestNewConfigMapVMPoolManagerNilConfig(t *testing.T) {
	client := fake.NewSimpleClientset()

	_, err := NewConfigMapVMPoolManager(client, nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}
	if err != ErrNilConfig {
		t.Errorf("Expected ErrNilConfig, got %v", err)
	}
}

func TestNewConfigMapVMPoolManagerEmptyIPs(t *testing.T) {
	config := &GlobalVMPoolConfig{
		PoolIPs: []string{},
	}

	client := fake.NewSimpleClientset()

	_, err := NewConfigMapVMPoolManager(client, config)
	if err == nil {
		t.Error("Expected error for empty IP list")
	}
}

func TestConfigMapVMPoolManagerAllocateIP(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11"},
		OperationTimeout: 10000, // 10 seconds in milliseconds for timeout
		SkipVMReadiness:  true,  // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// For testing basic allocation functionality, we don't need PeerPod service
	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test allocation
	allocationID := "test-allocation-1"
	allocatedIP, err := manager.AllocateIP(ctx, allocationID, "test-pod")
	if err != nil {
		t.Errorf("Failed to allocate IP: %v", err)
	}

	expectedIP, _ := netip.ParseAddr("192.168.1.10")
	if allocatedIP != expectedIP {
		t.Errorf("Expected allocated IP %s, got %s", expectedIP, allocatedIP)
	}

	// Verify ConfigMap was created
	configMaps := client.CoreV1().ConfigMaps(config.Namespace)
	cm, err := configMaps.Get(ctx, config.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	stateData, exists := cm.Data[stateDataKey]
	if !exists {
		t.Error("Expected state data in ConfigMap")
	}

	var state IPAllocationState
	err = json.Unmarshal([]byte(stateData), &state)
	if err != nil {
		t.Errorf("Failed to unmarshal state: %v", err)
	}

	if len(state.AllocatedIPs) != 1 {
		t.Errorf("Expected 1 allocated IP, got %d", len(state.AllocatedIPs))
	}

	if len(state.AvailableIPs) != 1 {
		t.Errorf("Expected 1 available IP, got %d", len(state.AvailableIPs))
	}

	allocation, exists := state.AllocatedIPs[allocationID]
	if !exists {
		t.Error("Expected allocation to exist")
	}

	if allocation.IP != allocatedIP.String() {
		t.Errorf("Expected allocation IP %s, got %s", allocatedIP.String(), allocation.IP)
	}
}

func TestConfigMapVMPoolManagerDeallocateIP(t *testing.T) {
	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Pre-create ConfigMap with allocated IP
	initialState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"test-allocation": {
				AllocationID: "test-allocation",
				IP:           "192.168.1.10",
				AllocatedAt:  metav1.Now(),
			},
		},
		AvailableIPs: []string{"192.168.1.11"},
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	stateData, _ := json.Marshal(initialState)
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
		t.Fatalf("Failed to create initial ConfigMap: %v", err)
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test deallocation
	err = manager.DeallocateIP(ctx, "test-allocation")
	if err != nil {
		t.Errorf("Failed to deallocate IP: %v", err)
	}

	// Verify state after deallocation
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

func TestConfigMapVMPoolManagerGetAllocatedIP(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()
	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Allocate an IP
	allocationID := "test-allocation"
	allocatedIP, err := manager.AllocateIP(ctx, allocationID, "test-pod")
	if err != nil {
		t.Fatalf("Failed to allocate IP: %v", err)
	}

	// Test GetIPfromAllocationID
	retrievedIP, exists, err := manager.GetIPfromAllocationID(ctx, allocationID)
	if err != nil {
		t.Errorf("Failed to get allocated IP: %v", err)
	}

	if !exists {
		t.Error("Expected allocation to exist")
	}

	if retrievedIP != allocatedIP {
		t.Errorf("Expected retrieved IP %s, got %s", allocatedIP, retrievedIP)
	}

	// Test non-existent allocation
	_, exists, err = manager.GetIPfromAllocationID(ctx, "non-existent")
	if err != nil {
		t.Errorf("Unexpected error for non-existent allocation: %v", err)
	}

	if exists {
		t.Error("Expected allocation to not exist")
	}
}

func TestConfigMapVMPoolManagerListAllocatedIPs(t *testing.T) {
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
	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Allocate two IPs
	alloc1 := "test-allocation-1"
	ip1, err := manager.AllocateIP(ctx, alloc1, "test-pod-1")
	if err != nil {
		t.Fatalf("Failed to allocate first IP: %v", err)
	}

	alloc2 := "test-allocation-2"
	ip2, err := manager.AllocateIP(ctx, alloc2, "test-pod-2")
	if err != nil {
		t.Fatalf("Failed to allocate second IP: %v", err)
	}

	// Test ListAllocatedIPs
	allocatedIPs, err := manager.ListAllocatedIPs(ctx)
	if err != nil {
		t.Errorf("Failed to list allocated IPs: %v", err)
	}

	if len(allocatedIPs) != 2 {
		t.Errorf("Expected 2 allocated IPs, got %d", len(allocatedIPs))
	}

	allocation1, exists := allocatedIPs[alloc1]
	if !exists {
		t.Error("Expected allocation1 to exist")
	}

	if allocation1.IP != ip1.String() {
		t.Errorf("Expected allocation1 IP %s, got %s", ip1.String(), allocation1.IP)
	}

	allocation2, exists := allocatedIPs[alloc2]
	if !exists {
		t.Error("Expected allocation2 to exist")
	}

	if allocation2.IP != ip2.String() {
		t.Errorf("Expected allocation2 IP %s, got %s", ip2.String(), allocation2.IP)
	}
}

func TestConfigMapVMPoolManagerInitializeEmptyState(t *testing.T) {
	config := &GlobalVMPoolConfig{
		Namespace:     "test-namespace",
		ConfigMapName: "test-configmap",
		PoolIPs:       []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
	}

	client := fake.NewSimpleClientset()
	manager := &ConfigMapVMPoolManager{
		client: client,
		config: config,
	}

	state := manager.initializeEmptyState()

	if len(state.AllocatedIPs) != 0 {
		t.Errorf("Expected 0 allocated IPs, got %d", len(state.AllocatedIPs))
	}

	if len(state.AvailableIPs) != 3 {
		t.Errorf("Expected 3 available IPs, got %d", len(state.AvailableIPs))
	}

	if state.Version != 1 {
		t.Errorf("Expected Version 1, got %d", state.Version)
	}

	// Verify all IPs are available
	expectedIPs := map[string]bool{
		"192.168.1.10": true,
		"192.168.1.11": true,
		"192.168.1.12": true,
	}

	for _, ip := range state.AvailableIPs {
		if !expectedIPs[ip] {
			t.Errorf("Unexpected IP in available list: %s", ip)
		}
		delete(expectedIPs, ip)
	}

	if len(expectedIPs) != 0 {
		t.Errorf("Expected all IPs to be in available list, missing: %v", expectedIPs)
	}
}

func TestConfigMapVMPoolManagerErrorHandling(t *testing.T) {
	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()

	// Add reactor to simulate ConfigMap creation failure
	client.PrependReactor("create", "configmaps", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.NewInternalError(fmt.Errorf("simulated creation failure"))
	})

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()

	// Test allocation failure due to ConfigMap creation error
	_, err = manager.AllocateIP(ctx, "test-allocation", "test-pod")
	if err == nil {
		t.Error("Expected error due to ConfigMap creation failure")
	}
}

func TestConfigMapVMPoolManagerDoubleAllocation(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	client := fake.NewSimpleClientset()
	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create ConfigMapVMPoolManager: %v", err)
	}

	ctx := context.Background()
	allocationID := "test-allocation"

	// First allocation should succeed
	ip1, err := manager.AllocateIP(ctx, allocationID, "test-pod")
	if err != nil {
		t.Fatalf("First allocation failed: %v", err)
	}

	// Second allocation with same ID should return same IP
	ip2, err := manager.AllocateIP(ctx, allocationID, "test-pod")
	if err != nil {
		t.Errorf("Second allocation failed: %v", err)
	}

	if ip1 != ip2 {
		t.Errorf("Expected same IP for double allocation, got %s and %s", ip1, ip2)
	}
}
