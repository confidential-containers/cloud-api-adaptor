// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNodeSpecificStateRecovery(t *testing.T) {
	// Set up test with specific node name
	originalNodeName := os.Getenv("NODE_NAME")
	defer func() {
		if originalNodeName != "" {
			os.Setenv("NODE_NAME", originalNodeName)
		} else {
			os.Unsetenv("NODE_NAME")
		}
	}()

	config := &GlobalVMPoolConfig{
		Namespace:        "test-namespace",
		ConfigMapName:    "test-configmap",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"},
		OperationTimeout: 10000,
	}

	client := fake.NewSimpleClientset()

	// Create initial state with allocations from multiple nodes
	initialState := &IPAllocationState{
		AllocatedIPs: map[string]IPAllocation{
			"pod1-sandbox1": {
				AllocationID: "pod1-sandbox1",
				IP:           "192.168.1.10",
				NodeName:     "node-1",
				PodName:      "pod1",
				AllocatedAt:  metav1.Now(),
			},
			"pod2-sandbox1": {
				AllocationID: "pod2-sandbox1",
				IP:           "192.168.1.11",
				NodeName:     "node-2",
				PodName:      "pod2",
				AllocatedAt:  metav1.Now(),
			},
		},
		AvailableIPs: []string{"192.168.1.12"},
		LastUpdated:  metav1.Now(),
		Version:      1,
	}

	// Create ConfigMap with existing state
	stateBytes, _ := json.MarshalIndent(initialState, "", "  ")
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			stateDataKey: string(stateBytes),
		},
	}
	_, err := client.CoreV1().ConfigMaps(config.Namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Simulate CAA restart on node-1
	os.Setenv("NODE_NAME", "node-1")

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Trigger state recovery
	ctx := context.Background()
	err = manager.RecoverState(ctx, nil)
	if err != nil {
		t.Fatalf("State recovery failed: %v", err)
	}

	// Verify that all allocations are preserved (peerpod controller handles cleanup)
	allocatedIPs, err := manager.ListAllocatedIPs(ctx)
	if err != nil {
		t.Fatalf("Failed to list allocated IPs: %v", err)
	}

	// Should have both allocations remaining (CAA doesn't release during recovery)
	if len(allocatedIPs) != 2 {
		t.Errorf("Expected 2 remaining allocations, got %d", len(allocatedIPs))
	}

	// Verify both allocations are preserved
	node1Found := false
	node2Found := false
	for allocID, allocation := range allocatedIPs {
		if allocation.NodeName == "node-1" && allocation.IP == "192.168.1.10" {
			node1Found = true
			t.Logf("Found preserved node-1 allocation: %s -> %s", allocID, allocation.IP)
		}
		if allocation.NodeName == "node-2" && allocation.IP == "192.168.1.11" {
			node2Found = true
			t.Logf("Found preserved node-2 allocation: %s -> %s", allocID, allocation.IP)
		}
	}

	if !node1Found {
		t.Error("Expected node-1 allocation to be preserved")
	}
	if !node2Found {
		t.Error("Expected node-2 allocation to be preserved")
	}

	// Verify pool status reflects all allocations preserved
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get pool status: %v", err)
	}

	expectedTotal := 3
	expectedAvailable := 1 // Only 192.168.1.12 (originally available)
	expectedInUse := 2     // Both node-1 and node-2 allocations preserved

	if total != expectedTotal {
		t.Errorf("Expected %d total IPs, got %d", expectedTotal, total)
	}
	if available != expectedAvailable {
		t.Errorf("Expected %d available IPs, got %d", expectedAvailable, available)
	}
	if inUse != expectedInUse {
		t.Errorf("Expected %d IPs in use, got %d", expectedInUse, inUse)
	}

	t.Logf("Node-specific recovery test passed: all allocations preserved for peerpod controller cleanup")
}
