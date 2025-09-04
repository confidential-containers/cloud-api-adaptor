// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"fmt"
	"net/netip"
	"sync"
)

// VMPool manages a simple pool of IP addresses for pre-created VMs
type VMPool struct {
	availableIPs []netip.Addr          // Available IP addresses
	allocatedIPs map[string]netip.Addr // allocationID -> allocated IP
	mu           sync.RWMutex          // Protects concurrent access
}

// NewVMPool creates and initializes a new VM pool from IP addresses
func NewVMPool(ipList []string) (*VMPool, error) {
	if len(ipList) == 0 {
		return nil, fmt.Errorf("vm-pool-ips is required and cannot be empty")
	}

	pool := &VMPool{
		availableIPs: make([]netip.Addr, 0, len(ipList)),
		allocatedIPs: make(map[string]netip.Addr),
	}

	// Parse and validate all IP addresses
	for _, ipStr := range ipList {
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			return nil, fmt.Errorf("invalid IP address %q: %w", ipStr, err)
		}
		pool.availableIPs = append(pool.availableIPs, addr)
	}

	logger.Printf("Initialized VM pool with %d IP addresses: %v", len(pool.availableIPs), ipList)
	return pool, nil
}

// AllocateIP allocates an available IP from the pool
func (pool *VMPool) AllocateIP(allocationID string) (netip.Addr, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Check if already allocated
	if ip, exists := pool.allocatedIPs[allocationID]; exists {
		return ip, fmt.Errorf("allocation ID %s already has IP %s", allocationID, ip.String())
	}

	// Check if any IPs are available
	if len(pool.availableIPs) == 0 {
		return netip.Addr{}, fmt.Errorf("no available IPs in pool")
	}

	// Allocate the first available IP
	ip := pool.availableIPs[0]
	pool.availableIPs = pool.availableIPs[1:]
	pool.allocatedIPs[allocationID] = ip

	logger.Printf("Allocated IP %s to allocation ID %s", ip.String(), allocationID)
	return ip, nil
}

// DeallocateByAllocationID returns an IP back to the pool by allocation ID
func (pool *VMPool) DeallocateByAllocationID(allocationID string) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	ip, exists := pool.allocatedIPs[allocationID]
	if !exists {
		return fmt.Errorf("allocation ID %s not found", allocationID)
	}

	// Return IP to available pool
	pool.availableIPs = append(pool.availableIPs, ip)
	delete(pool.allocatedIPs, allocationID)

	logger.Printf("Deallocated IP %s from allocation ID %s", ip.String(), allocationID)
	return nil
}

// DeallocateByIP returns an IP back to the pool by IP address
func (pool *VMPool) DeallocateByIP(ipAddr netip.Addr) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Find the allocation ID for this IP
	var allocationID string
	for id, ip := range pool.allocatedIPs {
		if ip == ipAddr {
			allocationID = id
			break
		}
	}

	if allocationID == "" {
		return fmt.Errorf("IP %s not found in allocated pool", ipAddr.String())
	}

	// Return IP to available pool
	pool.availableIPs = append(pool.availableIPs, ipAddr)
	delete(pool.allocatedIPs, allocationID)

	logger.Printf("Deallocated IP %s (was allocated to %s)", ipAddr.String(), allocationID)
	return nil
}

// GetStatus returns the current pool status
func (pool *VMPool) GetStatus() (total, available, inUse int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	total = len(pool.availableIPs) + len(pool.allocatedIPs)
	available = len(pool.availableIPs)
	inUse = len(pool.allocatedIPs)

	return total, available, inUse
}

// GetAllocatedIP returns the IP allocated to a specific allocation ID
func (pool *VMPool) GetAllocatedIP(allocationID string) (netip.Addr, bool) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	ip, exists := pool.allocatedIPs[allocationID]
	return ip, exists
}

// ListAllocatedIPs returns a copy of all currently allocated IPs
func (pool *VMPool) ListAllocatedIPs() map[string]netip.Addr {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	result := make(map[string]netip.Addr, len(pool.allocatedIPs))
	for id, ip := range pool.allocatedIPs {
		result[id] = ip
	}
	return result
}
