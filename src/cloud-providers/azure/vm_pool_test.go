// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"net/netip"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

func TestVMPoolAllocation(t *testing.T) {
	// Create a VM pool with mock data
	pool := &VMPool{
		config: VMPoolConfig{Type: VMPoolGlobal},
		entries: []VMPoolEntry{
			{
				Instance: provider.Instance{
					ID:   "/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/virtualMachines/vm1",
					Name: "vm1",
					IPs:  []netip.Addr{netip.MustParseAddr("10.0.1.10")},
				},
				InstanceType: "Standard_D2s_v3",
				InUse:        false,
				AllocatedTo:  "",
			},
			{
				Instance: provider.Instance{
					ID:   "/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/virtualMachines/vm2",
					Name: "vm2", 
					IPs:  []netip.Addr{netip.MustParseAddr("10.0.1.11")},
				},
				InstanceType: "Standard_D2s_v3",
				InUse:        false,
				AllocatedTo:  "",
			},
		},
		instanceTypeMap: make(map[string][]*VMPoolEntry),
	}

	// Test allocation
	vm1, err := pool.AllocateVM("test-pod-1", "")
	if err != nil {
		t.Fatalf("Failed to allocate VM: %v", err)
	}

	if vm1.Name != "vm1" {
		t.Errorf("Expected vm1, got %s", vm1.Name)
	}

	// Check pool status
	total, available, inUse := pool.GetPoolStatus()
	if total != 2 || available != 1 || inUse != 1 {
		t.Errorf("Expected pool status: total=2, available=1, inUse=1, got total=%d, available=%d, inUse=%d", 
			total, available, inUse)
	}

	// Allocate second VM
	vm2, err := pool.AllocateVM("test-pod-2", "")
	if err != nil {
		t.Fatalf("Failed to allocate second VM: %v", err)
	}

	if vm2.Name != "vm2" {
		t.Errorf("Expected vm2, got %s", vm2.Name)
	}

	// Try to allocate when pool is exhausted
	_, err = pool.AllocateVM("test-pod-3", "")
	if err == nil {
		t.Error("Expected error when pool is exhausted")
	}

	// Test deallocation by VM ID
	err = pool.DeallocateByVMID(vm1.ID)
	if err != nil {
		t.Fatalf("Failed to deallocate VM: %v", err)
	}

	// Check pool status after deallocation
	total, available, inUse = pool.GetPoolStatus()
	if total != 2 || available != 1 || inUse != 1 {
		t.Errorf("Expected pool status after deallocation: total=2, available=1, inUse=1, got total=%d, available=%d, inUse=%d", 
			total, available, inUse)
	}

	// Should be able to allocate again
	vm3, err := pool.AllocateVM("test-pod-3", "")
	if err != nil {
		t.Fatalf("Failed to allocate VM after deallocation: %v", err)
	}

	if vm3.Name != "vm1" {
		t.Errorf("Expected vm1 to be reallocated, got %s", vm3.Name)
	}
}

func TestVMPoolConfiguration(t *testing.T) {
	config := &Config{
		VMPoolType: "global",
		VMPoolIPs:  []string{"10.0.1.10", "10.0.1.11", "192.168.1.20"},
	}

	if len(config.VMPoolIPs) != 3 {
		t.Errorf("Expected 3 IPs, got %d", len(config.VMPoolIPs))
	}

	expectedIPs := []string{"10.0.1.10", "10.0.1.11", "192.168.1.20"}
	for i, ip := range config.VMPoolIPs {
		if ip != expectedIPs[i] {
			t.Errorf("Expected IP %s, got %s", expectedIPs[i], ip)
		}
	}
}

func TestPreCreatedIPsFlag(t *testing.T) {
	flag := &preCreatedIPsFlag{}
	
	// Test setting comma-separated values
	err := flag.Set("10.0.1.10,10.0.1.11, 192.168.1.20")
	if err != nil {
		t.Fatalf("Failed to set flag: %v", err)
	}

	if len(*flag) != 3 {
		t.Errorf("Expected 3 IPs, got %d", len(*flag))
	}

	expectedIPs := []string{"10.0.1.10", "10.0.1.11", "192.168.1.20"}
	for i, ip := range *flag {
		if ip != expectedIPs[i] {
			t.Errorf("Expected IP %s, got %s", expectedIPs[i], ip)
		}
	}

	// Test string representation
	str := flag.String()
	expected := "10.0.1.10,10.0.1.11,192.168.1.20"
	if str != expected {
		t.Errorf("Expected string %s, got %s", expected, str)
	}
}

func TestVMPoolTypeConfiguration(t *testing.T) {
	// Test global mode
	globalConfig := VMPoolConfig{
		Type:      VMPoolGlobal,
		IPs:       []string{"10.0.1.10", "10.0.1.11"},
	}

	if globalConfig.Type != VMPoolGlobal {
		t.Errorf("Expected VMPoolGlobal, got %s", globalConfig.Type)
	}

	// Test podregex mode
	regexConfig := VMPoolConfig{
		Type:     VMPoolPodRegex,
		PodRegex: ".*gpu.*",
		IPs:      []string{"10.0.1.10", "10.0.1.11"},
	}

	if regexConfig.Type != VMPoolPodRegex {
		t.Errorf("Expected VMPoolPodRegex, got %s", regexConfig.Type)
	}

	// Test instancetypes mode
	instanceConfig := VMPoolConfig{
		Type:          VMPoolInstanceType,
		InstanceTypes: []string{"Standard_NC6s_v3", "Standard_ND40rs_v2"},
		IPs:           []string{"10.0.1.10", "10.0.1.11", "10.0.1.12"},
	}

	if instanceConfig.Type != VMPoolInstanceType {
		t.Errorf("Expected VMPoolInstanceType, got %s", instanceConfig.Type)
	}

	if len(instanceConfig.InstanceTypes) != 2 {
		t.Errorf("Expected 2 instance types, got %d", len(instanceConfig.InstanceTypes))
	}
}