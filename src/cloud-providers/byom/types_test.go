// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultGlobalVMPoolConfig(t *testing.T) {
	config := DefaultGlobalVMPoolConfig()

	if config.Namespace != "confidential-containers-system" {
		t.Errorf("Expected namespace 'confidential-containers-system', got %s", config.Namespace)
	}

	if config.ConfigMapName != "byom-ip-pool-state" {
		t.Errorf("Expected ConfigMapName 'byom-ip-pool-state', got %s", config.ConfigMapName)
	}

	if config.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries 5, got %d", config.MaxRetries)
	}

	if config.RetryInterval != 100*time.Millisecond {
		t.Errorf("Expected RetryInterval 100ms, got %v", config.RetryInterval)
	}

	if config.OperationTimeout != 30*time.Second {
		t.Errorf("Expected OperationTimeout 30s, got %v", config.OperationTimeout)
	}
}

func TestIPAllocation(t *testing.T) {
	now := metav1.Now()
	allocation := IPAllocation{
		AllocationID: "test-pod-sandbox1",
		IP:           "192.168.1.10",
		NodeName:     "test-node",
		PodName:      "test-pod",
		PodNamespace: "test-namespace",
		AllocatedAt:  now,
	}

	if allocation.AllocationID != "test-pod-sandbox1" {
		t.Errorf("Expected AllocationID 'test-pod-sandbox1', got %s", allocation.AllocationID)
	}

	if allocation.IP != "192.168.1.10" {
		t.Errorf("Expected IP '192.168.1.10', got %s", allocation.IP)
	}

	if allocation.NodeName != "test-node" {
		t.Errorf("Expected NodeName 'test-node', got %s", allocation.NodeName)
	}

	if allocation.PodName != "test-pod" {
		t.Errorf("Expected PodName 'test-pod', got %s", allocation.PodName)
	}

	if allocation.PodNamespace != "test-namespace" {
		t.Errorf("Expected PodNamespace 'test-namespace', got %s", allocation.PodNamespace)
	}

	if allocation.AllocatedAt != now {
		t.Errorf("Expected AllocatedAt %v, got %v", now, allocation.AllocatedAt)
	}
}

func TestIPAllocationState(t *testing.T) {
	state := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"test-1": {
				AllocationID: "test-1",
				IP:           "192.168.1.10",
				NodeName:     "test-node",
				PodName:      "test-pod",
				PodNamespace: "test-namespace",
				AllocatedAt:  metav1.Now(),
			},
		},
		AvailableIPs: []string{"192.168.1.11", "192.168.1.12"},
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	if len(state.AllocatedIPs) != 1 {
		t.Errorf("Expected 1 allocated IP, got %d", len(state.AllocatedIPs))
	}

	if len(state.AvailableIPs) != 2 {
		t.Errorf("Expected 2 available IPs, got %d", len(state.AvailableIPs))
	}

	if state.Version != 1 {
		t.Errorf("Expected Version 1, got %d", state.Version)
	}

	// Test allocation lookup
	allocation, exists := state.AllocatedIPs["test-1"]
	if !exists {
		t.Error("Expected allocation 'test-1' to exist")
	}

	if allocation.IP != "192.168.1.10" {
		t.Errorf("Expected allocated IP '192.168.1.10', got %s", allocation.IP)
	}
}
