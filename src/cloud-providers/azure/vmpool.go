// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"sync"

	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

// VMPoolType represents different VM pool allocation strategies
type VMPoolType string

const (
	VMPoolDisabled     VMPoolType = "disabled"
	VMPoolGlobal       VMPoolType = "global"
	VMPoolPodRegex     VMPoolType = "podregex"
	VMPoolInstanceType VMPoolType = "instancetypes"
)

// VMPoolConfig holds the VM pool configuration
type VMPoolConfig struct {
	Type          VMPoolType
	PodRegex      string
	InstanceTypes []string // List of instance types to use pool for
	IPs           []string // Pre-created VM IPs
	compiledRegex *regexp.Regexp
}

// VMPoolEntry represents a VM in the pool
type VMPoolEntry struct {
	Instance     provider.Instance
	InstanceType string // Detected instance type from Azure
	InUse        bool
	AllocatedTo  string
}

// VMPool manages pre-created VMs for different allocation strategies
type VMPool struct {
	config          VMPoolConfig
	entries         []VMPoolEntry
	instanceTypeMap map[string][]*VMPoolEntry // Dynamically built mapping
	mu              sync.RWMutex
}

// NewVMPool creates and initializes a new VM pool
func NewVMPool(config VMPoolConfig, azureProvider *azureProvider) (*VMPool, error) {
	if config.Type == VMPoolDisabled {
		return nil, nil
	}

	if len(config.IPs) == 0 {
		return nil, fmt.Errorf("vm-pool-ips is required for VM pool mode %s", config.Type)
	}

	pool := &VMPool{
		config:          config,
		instanceTypeMap: make(map[string][]*VMPoolEntry),
	}

	// Compile regex if needed
	if config.Type == VMPoolPodRegex && config.PodRegex != "" {
		compiledRegex, err := regexp.Compile(config.PodRegex)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pod regex %q: %w", config.PodRegex, err)
		}
		pool.config.compiledRegex = compiledRegex
	}

	// Initialize VM pool - this will dynamically detect instance types
	if err := pool.initVMPool(azureProvider); err != nil {
		return nil, fmt.Errorf("failed to initialize VM pool: %w", err)
	}

	return pool, nil
}

// initVMPool queries VMs by IPs and dynamically creates instance type mapping
func (pool *VMPool) initVMPool(azureProvider *azureProvider) error {
	// Query all VMs by their IPs
	instances, vmDetails, err := azureProvider.queryVMsWithDetailsByIPs(pool.config.IPs)
	if err != nil {
		return fmt.Errorf("failed to query VMs by IPs: %w", err)
	}

	pool.entries = make([]VMPoolEntry, len(instances))

	for i, vm := range instances {
		// Get the instance type from VM details
		instanceType := ""
		if i < len(vmDetails) && vmDetails[i].Properties != nil && vmDetails[i].Properties.HardwareProfile != nil {
			instanceType = string(*vmDetails[i].Properties.HardwareProfile.VMSize)
		}

		entry := VMPoolEntry{
			Instance:     vm,
			InstanceType: instanceType,
			InUse:        false,
			AllocatedTo:  "",
		}
		pool.entries[i] = entry

		// Add to instance type map for quick lookup
		if instanceType != "" {
			pool.instanceTypeMap[instanceType] = append(pool.instanceTypeMap[instanceType], &pool.entries[i])
		}

		logger.Printf("Added VM to pool: ID=%s, Name=%s, InstanceType=%s, IPs=%v",
			vm.ID, vm.Name, instanceType, vm.IPs)
	}

	// Log the discovered instance type mapping
	logger.Printf("VM Pool initialized with %d VMs across %d instance types:",
		len(pool.entries), len(pool.instanceTypeMap))
	for instanceType, entries := range pool.instanceTypeMap {
		logger.Printf("  Instance type %s: %d VMs", instanceType, len(entries))
	}

	// For instancetypes mode, validate that we have VMs for the specified instance types
	if pool.config.Type == VMPoolInstanceType {
		return pool.validateInstanceTypes()
	}

	return nil
}

// validateInstanceTypes ensures that specified instance types have VMs in the pool
func (pool *VMPool) validateInstanceTypes() error {
	if len(pool.config.InstanceTypes) == 0 {
		return fmt.Errorf("vm-pool-instance-types is required for instancetypes mode")
	}

	var missingTypes []string
	for _, requiredType := range pool.config.InstanceTypes {
		if _, exists := pool.instanceTypeMap[requiredType]; !exists {
			missingTypes = append(missingTypes, requiredType)
		}
	}

	if len(missingTypes) > 0 {
		return fmt.Errorf("no VMs found for required instance types: %s", strings.Join(missingTypes, ", "))
	}

	logger.Printf("Instance type validation successful. Available types: %s",
		strings.Join(pool.config.InstanceTypes, ", "))
	return nil
}

// ShouldUsePool determines if a pod should use the VM pool based on configuration
func (pool *VMPool) ShouldUsePool(podName string, requestedInstanceType string) bool {

	switch pool.config.Type {
	case VMPoolDisabled:
		logger.Printf("VM pool is disabled")
		return false
	case VMPoolGlobal:
		logger.Printf("VM pool is enabled globally")
		return true
	case VMPoolPodRegex:
		if pool.config.compiledRegex != nil {
			if pool.config.compiledRegex.MatchString(podName) {
				logger.Printf("Pod %q matches VM pool pod regex", podName)
				return true
			}
		}
		logger.Printf("Pod %q does not match VM pool pod regex", podName)
		return false
	case VMPoolInstanceType:
		// Check if the requested instance type is in our configured list
		for _, configuredType := range pool.config.InstanceTypes {
			if configuredType == requestedInstanceType {
				logger.Printf("Requested instance type %q is configured to use VM pool", requestedInstanceType)
				return true
			}
		}
		logger.Printf("Requested instance type %q is not configured to use VM pool", requestedInstanceType)
		return false
	default:
		return false
	}
}

// AllocateVM allocates a VM from the pool
func (pool *VMPool) AllocateVM(allocationID string, preferredInstanceType string) (*provider.Instance, error) {

	pool.mu.Lock()
	defer pool.mu.Unlock()

	// For instance type mode, try to find a VM with matching instance type first
	if pool.config.Type == VMPoolInstanceType && preferredInstanceType != "" {
		if entries, exists := pool.instanceTypeMap[preferredInstanceType]; exists {
			for _, entry := range entries {
				if !entry.InUse {
					entry.InUse = true
					entry.AllocatedTo = allocationID
					logger.Printf("Allocated VM from pool (instance type %s): ID=%s, Name=%s, IPs=%v",
						preferredInstanceType, entry.Instance.ID, entry.Instance.Name, entry.Instance.IPs)
					return &entry.Instance, nil
				}
			}
			return nil, fmt.Errorf("no available VMs of instance type %s in pool", preferredInstanceType)
		}
	}

	// Fallback: find any available VM (for global/podregex modes)
	for i := range pool.entries {
		if !pool.entries[i].InUse {
			pool.entries[i].InUse = true
			pool.entries[i].AllocatedTo = allocationID
			logger.Printf("Allocated VM from pool: ID=%s, Name=%s, IPs=%v",
				pool.entries[i].Instance.ID, pool.entries[i].Instance.Name, pool.entries[i].Instance.IPs)
			return &pool.entries[i].Instance, nil
		}
	}

	return nil, fmt.Errorf("no available VMs in pool")
}

// DeallocateByVMID marks a VM as available in the pool by its VM ID
// The VM is preserved and not deleted from Azure
func (pool *VMPool) DeallocateByVMID(vmID string) error {

	pool.mu.Lock()
	defer pool.mu.Unlock()

	for i := range pool.entries {
		if pool.entries[i].Instance.ID == vmID && pool.entries[i].InUse {
			pool.entries[i].InUse = false
			pool.entries[i].AllocatedTo = ""
			logger.Printf("VM marked as available in pool: ID=%s", vmID)
			return nil
		}
	}
	return fmt.Errorf("VM not found or not allocated: %s", vmID)
}

// DeallocateByAllocationID marks a VM as available in the pool by its allocation ID
// The VM is preserved and not deleted from Azure
func (pool *VMPool) DeallocateByAllocationID(allocationID string) error {

	pool.mu.Lock()
	defer pool.mu.Unlock()

	for i := range pool.entries {
		if pool.entries[i].AllocatedTo == allocationID && pool.entries[i].InUse {
			pool.entries[i].InUse = false
			pool.entries[i].AllocatedTo = ""
			logger.Printf("VM marked as available in pool by allocation ID: %s", allocationID)
			return nil
		}
	}
	return fmt.Errorf("VM not found for allocation ID: %s", allocationID)
}

// GetPoolStatus returns the current pool status
func (pool *VMPool) GetPoolStatus() (total, available, inUse int) {

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	total = len(pool.entries)
	for _, entry := range pool.entries {
		if entry.InUse {
			inUse++
		} else {
			available++
		}
	}
	return
}

// GetPoolStatusByInstanceType returns pool status grouped by instance type
func (pool *VMPool) GetPoolStatusByInstanceType() map[string]struct{ Total, Available, InUse int } {

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make(map[string]struct{ Total, Available, InUse int })

	for _, entry := range pool.entries {
		instanceType := entry.InstanceType
		if instanceType == "" {
			instanceType = "unknown"
		}

		current := status[instanceType]
		current.Total++
		if entry.InUse {
			current.InUse++
		} else {
			current.Available++
		}
		status[instanceType] = current
	}

	return status
}

// AllocateFromVMPool allocates a VM from the pool (used by provider.go)
// The caller must check if pool must be used or not by calling ShouldUsePool
func (p *azureProvider) AllocateFromVMPool(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, requestedInstanceType string) (*provider.Instance, error) {
	if p.vmPool == nil {
		return nil, fmt.Errorf("VM pool is not initialized")
	}
	// Generate allocation ID
	allocationID := fmt.Sprintf("%s-%s", podName, sandboxID)

	// Allocate VM from pool
	vm, err := p.vmPool.AllocateVM(allocationID, requestedInstanceType)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate VM from pool: %w", err)
	}

	// Generate and send cloud config to the pre-created VM
	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		// Rollback allocation on error
		p.vmPool.DeallocateByAllocationID(allocationID)
		return nil, fmt.Errorf("failed to generate cloud config: %w", err)
	}

	// Send config to the pre-created VM via SFTP
	if len(vm.IPs) > 0 {
		err = p.sendConfigFile(ctx, cloudConfigData, vm.IPs[0])
		if err != nil {
			// Rollback allocation on error
			p.vmPool.DeallocateByAllocationID(allocationID)
			return nil, fmt.Errorf("failed to send config to VM: %w", err)
		}
	}

	// Set pool metadata for the returned instance
	vm.PoolMetadata = &provider.PoolMetadata{
		AllocationID: allocationID,
		PoolType:     string(p.vmPool.config.Type),
	}

	// Log current pool status
	total, available, inUse := p.vmPool.GetPoolStatus()
	logger.Printf("Pool status after allocation: total=%d, available=%d, inUse=%d", total, available, inUse)

	return vm, nil
}

// DeallocateFromVMPool returns a VM back to the pool for reuse (used by provider.go)
// IMPORTANT: This does NOT delete the VM from Azure, it only marks it as available in the pool
func (p *azureProvider) DeallocateFromVMPool(instanceID string) error {
	if p.vmPool == nil {
		return fmt.Errorf("VM pool is not initialized")
	}

	err := p.vmPool.DeallocateByVMID(instanceID)
	if err != nil {
		return fmt.Errorf("failed to deallocate VM from pool: %w", err)
	}

	logger.Printf("VM returned to pool for reuse (not deleted): ID=%s", instanceID)

	// Log current pool status
	total, available, inUse := p.vmPool.GetPoolStatus()
	logger.Printf("Pool status after deallocation: total=%d, available=%d, inUse=%d", total, available, inUse)

	return nil
}

// queryVMsWithDetailsByIPs queries Azure to find VMs by their IP addresses and returns both instances and VM details
func (p *azureProvider) queryVMsWithDetailsByIPs(ipList []string) ([]provider.Instance, []*armcompute.VirtualMachine, error) {
	vmClient, err := armcompute.NewVirtualMachinesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create VM client: %w", err)
	}

	nicClient, err := armnetwork.NewInterfacesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create network interfaces client: %w", err)
	}

	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create public IP client: %w", err)
	}

	rgName := p.serviceConfig.ResourceGroupName
	var instances []provider.Instance
	var vmDetails []*armcompute.VirtualMachine

	// Convert IP strings to a map for quick lookup
	targetIPs := make(map[string]bool)
	for _, ip := range ipList {
		addr, err := netip.ParseAddr(strings.TrimSpace(ip))
		if err != nil {
			return nil, nil, fmt.Errorf("invalid IP address %s: %w", ip, err)
		}
		targetIPs[addr.String()] = true
	}

	// List all VMs in the resource group
	vmPager := vmClient.NewListPager(rgName, nil)
	for vmPager.More() {
		page, err := vmPager.NextPage(context.Background())
		if err != nil {
			return nil, nil, fmt.Errorf("list VMs: %w", err)
		}

		for _, vm := range page.Value {
			if vm.Properties == nil || vm.Properties.NetworkProfile == nil {
				continue
			}

			// Get VM's IP addresses
			vmIPs, err := p.getVMIPsFromNetworkProfile(nicClient, publicIPClient, vm.Properties.NetworkProfile.NetworkInterfaces, rgName)
			if err != nil {
				logger.Printf("Failed to get IPs for VM %s: %v", *vm.Name, err)
				continue
			}

			// Check if any of the VM's IPs match our target IPs
			var matchingIPs []netip.Addr
			for _, vmIP := range vmIPs {
				if targetIPs[vmIP.String()] {
					matchingIPs = append(matchingIPs, vmIP)
				}
			}

			// If we found matching IPs, add this VM to our instances
			if len(matchingIPs) > 0 {
				instances = append(instances, provider.Instance{
					ID:   *vm.ID,
					Name: *vm.Name,
					IPs:  matchingIPs,
				})
				vmDetails = append(vmDetails, vm)
				logger.Printf("Found VM: %s (ID: %s) with matching IPs: %v", *vm.Name, *vm.ID, matchingIPs)
			}
		}
	}

	if len(instances) != len(ipList) {
		return nil, nil, fmt.Errorf("found %d VMs but expected %d (based on IP list)", len(instances), len(ipList))
	}

	return instances, vmDetails, nil
}
