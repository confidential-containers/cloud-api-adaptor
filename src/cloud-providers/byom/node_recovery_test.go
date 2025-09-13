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
				PodNamespace: "default",
				AllocatedAt:  metav1.Now(),
			},
			"pod2-sandbox1": {
				AllocationID: "pod2-sandbox1",
				IP:           "192.168.1.11",
				NodeName:     "node-2",
				PodName:      "pod2",
				PodNamespace: "kube-system",
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
	err = manager.RecoverState(ctx)
	if err != nil {
		t.Fatalf("State recovery failed: %v", err)
	}

	// Verify that node-1's allocation was released but node-2's was kept
	allocatedIPs, err := manager.ListAllocatedIPs(ctx)
	if err != nil {
		t.Fatalf("Failed to list allocated IPs: %v", err)
	}

	// Should only have node-2's allocation remaining
	if len(allocatedIPs) != 1 {
		t.Errorf("Expected 1 remaining allocation, got %d", len(allocatedIPs))
	}

	for allocID, allocation := range allocatedIPs {
		if allocation.NodeName != "node-2" {
			t.Errorf("Expected allocation from node-2, got allocation %s from node %s", allocID, allocation.NodeName)
		}
		if allocation.IP != "192.168.1.11" {
			t.Errorf("Expected IP 192.168.1.11, got %s", allocation.IP)
		}
	}

	// Verify that node-1's IP was returned to available pool
	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get pool status: %v", err)
	}

	expectedTotal := 3
	expectedAvailable := 2 // 192.168.1.10 (released) + 192.168.1.12 (originally available)
	expectedInUse := 1     // node-2's allocation

	if total != expectedTotal {
		t.Errorf("Expected %d total IPs, got %d", expectedTotal, total)
	}
	if available != expectedAvailable {
		t.Errorf("Expected %d available IPs, got %d", expectedAvailable, available)
	}
	if inUse != expectedInUse {
		t.Errorf("Expected %d IPs in use, got %d", expectedInUse, inUse)
	}

	t.Logf("Node-specific recovery test passed: node-1 IP released, node-2 IP preserved")
}
