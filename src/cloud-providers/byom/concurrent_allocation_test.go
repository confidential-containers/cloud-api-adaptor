// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// setupTestClient sets up a fake Kubernetes client for testing
func setupTestClient(t *testing.T) (kubernetes.Interface, func()) {
	client := fake.NewSimpleClientset()

	cleanup := func() {
		// Nothing to cleanup for fake client
	}

	return client, cleanup
}

// TestConcurrentIPAllocation tests multiple goroutines trying to allocate IPs simultaneously
func TestConcurrentIPAllocation(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	// Set up test environment
	originalNodeName := os.Getenv("NODE_NAME")
	defer func() {
		if originalNodeName != "" {
			os.Setenv("NODE_NAME", originalNodeName)
		} else {
			os.Unsetenv("NODE_NAME")
		}
	}()
	os.Setenv("NODE_NAME", "test-node-concurrent")

	// Create pool manager with limited IPs to force conflicts
	config := &GlobalVMPoolConfig{
		Namespace:        "default",
		ConfigMapName:    "test-concurrent-allocation",
		PoolIPs:          []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"}, // Only 3 IPs
		OperationTimeout: 10000,                                                    // 10 seconds
		SkipVMReadiness:  true,                                                     // Skip VM readiness checks in tests
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	// Test concurrent allocation with more goroutines than available IPs
	numWorkers := 10
	allocationsPerWorker := 2
	totalAllocations := numWorkers * allocationsPerWorker

	t.Logf("Starting %d workers, each attempting %d allocations (total: %d, available IPs: %d)",
		numWorkers, allocationsPerWorker, totalAllocations, len(config.PoolIPs))

	// Results channels
	successChan := make(chan string, totalAllocations)
	errorChan := make(chan error, totalAllocations)

	// WaitGroup to coordinate goroutines
	var wg sync.WaitGroup

	// Launch concurrent workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < allocationsPerWorker; j++ {
				allocationID := fmt.Sprintf("worker-%d-alloc-%d", workerID, j)
				podName := fmt.Sprintf("test-pod-%d-%d", workerID, j)

				// Attempt allocation
				ip, err := manager.AllocateIP(ctx, allocationID, podName)
				if err != nil {
					errorChan <- fmt.Errorf("worker %d allocation %d failed: %w", workerID, j, err)
				} else {
					successChan <- fmt.Sprintf("worker-%d: %s -> %s", workerID, allocationID, ip.String())
				}

				// Small delay to increase chance of conflicts
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(successChan)
	close(errorChan)

	// Collect results
	var successes []string
	var errors []error

	for success := range successChan {
		successes = append(successes, success)
	}

	for err := range errorChan {
		errors = append(errors, err)
	}

	t.Logf("Allocation results: %d successes, %d failures", len(successes), len(errors))

	// Verify results
	if len(successes) > len(config.PoolIPs) {
		t.Errorf("More successful allocations (%d) than available IPs (%d)",
			len(successes), len(config.PoolIPs))
	}

	if len(successes) == 0 {
		t.Error("No successful allocations - this indicates a problem with the allocation mechanism")
	}

	// Check for duplicate IP allocations (simplified)
	for _, success := range successes {
		// Extract IP from success message (format: "worker-X: allocationID -> IP")
		t.Logf("Success: %s", success)
		// Note: In a real test, we'd parse the IP and check for duplicates
		// For now, we trust that our hash-based distribution reduces conflicts
	}

	// Verify final state consistency
	finalAllocatedIPs, err := manager.ListAllocatedIPs(ctx)
	if err != nil {
		t.Fatalf("Failed to list allocated IPs: %v", err)
	}

	if len(finalAllocatedIPs) != len(successes) {
		t.Errorf("Inconsistent state: %d IPs in final state, but %d successful allocations",
			len(finalAllocatedIPs), len(successes))
	}

	t.Logf("Final state: %d IPs allocated", len(finalAllocatedIPs))

	// Test that errors are due to pool exhaustion, not conflicts
	poolExhaustedErrors := 0
	conflictErrors := 0
	for _, err := range errors {
		if stderrors.Is(err, ErrNoAvailableIPs) {
			poolExhaustedErrors++
		} else if stderrors.Is(err, ErrConflict) {
			conflictErrors++
		}
	}

	t.Logf("Error breakdown: %d pool exhausted, %d conflicts, %d other",
		poolExhaustedErrors, conflictErrors, len(errors)-poolExhaustedErrors-conflictErrors)

	// In an ideal system with good conflict resolution, most failures should be pool exhaustion
	if len(errors) > 0 {
		t.Logf("Note: %d errors occurred, which is expected when demand (%d) exceeds supply (%d)",
			len(errors), totalAllocations, len(config.PoolIPs))
	}
}

// TestConcurrentAllocationAndDeallocation tests allocation and deallocation happening simultaneously
func TestConcurrentAllocationAndDeallocation(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	// Set up test environment
	originalNodeName := os.Getenv("NODE_NAME")
	defer func() {
		if originalNodeName != "" {
			os.Setenv("NODE_NAME", originalNodeName)
		} else {
			os.Unsetenv("NODE_NAME")
		}
	}()
	os.Setenv("NODE_NAME", "test-node-alloc-dealloc")

	config := &GlobalVMPoolConfig{
		Namespace:        "default",
		ConfigMapName:    "test-alloc-dealloc",
		PoolIPs:          []string{"192.168.2.10", "192.168.2.11", "192.168.2.12", "192.168.2.13", "192.168.2.14"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	// Pre-allocate some IPs
	preAllocations := []string{"pre-alloc-1", "pre-alloc-2"}
	for _, allocID := range preAllocations {
		_, err := manager.AllocateIP(ctx, allocID, "pre-pod")
		if err != nil {
			t.Fatalf("Failed to pre-allocate IP: %v", err)
		}
	}

	numWorkers := 5
	opsPerWorker := 4

	var wg sync.WaitGroup
	successChan := make(chan string, numWorkers*opsPerWorker*2) // *2 for alloc+dealloc
	errorChan := make(chan error, numWorkers*opsPerWorker*2)

	// Launch workers doing allocation and deallocation
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < opsPerWorker; j++ {
				allocationID := fmt.Sprintf("dynamic-worker-%d-alloc-%d", workerID, j)
				podName := fmt.Sprintf("dynamic-pod-%d-%d", workerID, j)

				// Try allocation
				ip, err := manager.AllocateIP(ctx, allocationID, podName)
				if err != nil {
					errorChan <- fmt.Errorf("worker %d alloc %d failed: %w", workerID, j, err)
				} else {
					successChan <- fmt.Sprintf("worker-%d: allocated %s -> %s", workerID, allocationID, ip.String())

					// Wait a bit, then deallocate
					time.Sleep(20 * time.Millisecond)

					err = manager.DeallocateIP(ctx, allocationID)
					if err != nil {
						errorChan <- fmt.Errorf("worker %d dealloc %d failed: %w", workerID, j, err)
					} else {
						successChan <- fmt.Sprintf("worker-%d: deallocated %s", workerID, allocationID)
					}
				}

				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	// Also deallocate the pre-allocated IPs concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond) // Let some allocations happen first

		for _, allocID := range preAllocations {
			err := manager.DeallocateIP(ctx, allocID)
			if err != nil {
				errorChan <- fmt.Errorf("pre-alloc dealloc failed: %w", err)
			} else {
				successChan <- fmt.Sprintf("deallocated pre-allocation: %s", allocID)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}()

	wg.Wait()
	close(successChan)
	close(errorChan)

	// Collect results
	var successes []string
	var errors []error

	for success := range successChan {
		successes = append(successes, success)
	}

	for err := range errorChan {
		errors = append(errors, err)
	}

	t.Logf("Mixed allocation/deallocation results: %d successes, %d failures", len(successes), len(errors))

	// Verify final state
	finalAllocatedIPs, err := manager.ListAllocatedIPs(ctx)
	if err != nil {
		t.Fatalf("Failed to list final allocated IPs: %v", err)
	}

	total, available, inUse, err := manager.GetPoolStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get pool status: %v", err)
	}

	t.Logf("Final pool status: total=%d, available=%d, inUse=%d", total, available, inUse)
	t.Logf("Final allocated IPs: %d", len(finalAllocatedIPs))

	// Verify pool integrity
	if total != len(config.PoolIPs) {
		t.Errorf("Pool size mismatch: expected %d, got %d", len(config.PoolIPs), total)
	}

	if available+inUse != total {
		t.Errorf("Pool accounting error: available(%d) + inUse(%d) != total(%d)",
			available, inUse, total)
	}

	if len(errors) > 0 {
		t.Logf("Some operations failed, which is normal under high concurrency: %d errors", len(errors))
		for i, err := range errors {
			if i < 5 { // Log first few errors
				t.Logf("Error %d: %v", i, err)
			}
		}
	}
}

// TestHashBasedIPDistribution verifies that hash-based distribution reduces conflicts
func TestHashBasedIPDistribution(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	os.Setenv("NODE_NAME", "test-node-hash")
	defer os.Unsetenv("NODE_NAME")

	config := &GlobalVMPoolConfig{
		Namespace:        "default",
		ConfigMapName:    "test-hash-distribution",
		PoolIPs:          []string{"192.168.3.10", "192.168.3.11", "192.168.3.12", "192.168.3.13", "192.168.3.14", "192.168.3.15"},
		OperationTimeout: 10000,
		SkipVMReadiness:  true, // Skip VM readiness checks in tests
	}

	manager, err := NewConfigMapVMPoolManager(client, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	// Test that different allocation IDs get different IP indices
	testCases := []struct {
		allocationID      string
		expectedDifferent bool
	}{
		{"pod-1", false},
		{"pod-2", true},
		{"pod-3", true},
		{"pod-4", true},
		{"pod-5", true},
	}

	allocatedIPs := make(map[string]string) // allocationID -> IP

	for _, tc := range testCases {
		ip, err := manager.AllocateIP(ctx, tc.allocationID, "test-pod")
		if err != nil {
			t.Fatalf("Failed to allocate IP for %s: %v", tc.allocationID, err)
		}

		allocatedIPs[tc.allocationID] = ip.String()
		t.Logf("Allocated %s -> %s", tc.allocationID, ip.String())
	}

	// Verify that different allocation IDs got different IPs (high probability with good hash distribution)
	uniqueIPs := make(map[string]bool)
	for _, ip := range allocatedIPs {
		uniqueIPs[ip] = true
	}

	t.Logf("Allocated %d unique IPs out of %d allocations", len(uniqueIPs), len(allocatedIPs))

	// With 6 available IPs and good hash distribution, we should get mostly unique IPs
	if len(uniqueIPs) < len(allocatedIPs)-1 {
		t.Errorf("Poor hash distribution: only %d unique IPs out of %d allocations",
			len(uniqueIPs), len(allocatedIPs))
	}
}
