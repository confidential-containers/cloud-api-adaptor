// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIPAllocation(t *testing.T) {
	now := metav1.Now()
	allocation := IPAllocation{
		AllocationID: "test-pod-sandbox1",
		IP:           "192.168.1.10",
		NodeName:     "test-node",
		PodName:      "test-pod",
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

func TestVMPoolIPsValidation(t *testing.T) {
	var ips vmPoolIPs
	maxRangeIPs = 10

	// Test valid IPv4 addresses
	err := ips.Set("192.168.1.1,10.0.0.1,172.16.0.1")
	if err != nil {
		t.Errorf("Valid IPv4 addresses should be accepted: %v", err)
	}
	if len(ips) != 3 {
		t.Errorf("Expected 3 IPs, got %d", len(ips))
	}

	// Test valid IPv6 addresses
	err = ips.Set("2001:db8::1,::1,fe80::1")
	if err != nil {
		t.Errorf("Valid IPv6 addresses should be accepted: %v", err)
	}

	// Test mixed IPv4 and IPv6
	err = ips.Set("192.168.1.1,2001:db8::1,10.0.0.1")
	if err != nil {
		t.Errorf("Mixed IPv4 and IPv6 addresses should be accepted: %v", err)
	}

	// Test invalid IP address
	err = ips.Set("192.168.1.1,invalid-ip,10.0.0.1")
	if err == nil {
		t.Error("Invalid IP address should be rejected")
	}

	// Test malformed IP
	err = ips.Set("999.999.999.999")
	if err == nil {
		t.Error("Malformed IP address should be rejected")
	}

	// Test max range of IPs with maxRangeIPs=10
	err = ips.Set("10.1.1.1,192.168.1.1-192.168.1.11,10.1.1.4")
	if err != nil {
		t.Errorf("Valid IP range should be accepted: %v", err)
	}
	if len(ips) != 12 {
		t.Errorf("Expected 12 IPs, got %d", len(ips))
	}

	// Test IP ranges
	err = ips.Set("192.168.1.1-192.168.1.4,10.0.0.1")
	if err != nil {
		t.Errorf("Valid IP range should be accepted: %v", err)
	}
	if len(ips) != 5 {
		t.Errorf("Expected 5 IPs, got %d", len(ips))
	}

	// Test deduplication of IPs
	err = ips.Set("192.168.1.1-192.168.1.4,192.168.1.1")
	if err != nil {
		t.Errorf("Valid IP range should be accepted: %v", err)
	}
	if len(ips) != 4 {
		t.Errorf("Expected 4 IPs, got %d", len(ips))
	}

	// Test IP range validation
	err1 := ips.Set("192.168.1.4-192.168.1.2")
	err2 := ips.Set("192.168.1.1-192.168.1.1")
	if err1 == nil || err2 == nil {
		t.Error("Invalid IP range with start IP <= end IP")
	}

	// Test empty strings are skipped
	err = ips.Set("192.168.1.1, ,10.0.0.1,  ")
	if err != nil {
		t.Errorf("Empty strings should be skipped: %v", err)
	}
	if len(ips) != 2 {
		t.Errorf("Expected 2 IPs after skipping empty strings, got %d", len(ips))
	}

	// Test empty input
	err = ips.Set("")
	if err != nil {
		t.Errorf("Empty input should be accepted: %v", err)
	}
	if len(ips) != 0 {
		t.Errorf("Expected 0 IPs for empty input, got %d", len(ips))
	}

	// Test spaces are trimmed
	err = ips.Set(" 192.168.1.1 , 10.0.0.1 ")
	if err != nil {
		t.Errorf("Spaces should be trimmed: %v", err)
	}
	if ips[0] != "192.168.1.1" || ips[1] != "10.0.0.1" {
		t.Errorf("Expected trimmed IPs, got %v", ips)
	}
}
