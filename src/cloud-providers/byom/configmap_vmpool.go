// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	stateDataKey = "allocation-state"
	// Node identity detection paths
	nodeNameEnvVar = "NODE_NAME"
	nodeNameFile   = "/etc/podinfo/nodename"
	hostnameFile   = "/etc/hostname"
)

// ConfigMapVMPoolManager implements GlobalVMPoolManager using Kubernetes ConfigMap
type ConfigMapVMPoolManager struct {
	client kubernetes.Interface
	config *GlobalVMPoolConfig
	mutex  sync.RWMutex
}

// NewConfigMapVMPoolManager creates a new ConfigMap-based VM pool manager
func NewConfigMapVMPoolManager(client kubernetes.Interface, config *GlobalVMPoolConfig) (GlobalVMPoolManager, error) {
	if client == nil {
		return nil, ErrInvalidClient
	}

	if config == nil {
		return nil, ErrNilConfig
	}

	// Validate pool configuration
	if len(config.PoolIPs) == 0 {
		return nil, ErrEmptyPoolIPs
	}

	// Validate IP addresses
	for _, ipStr := range config.PoolIPs {
		if _, err := netip.ParseAddr(ipStr); err != nil {
			return nil, fmt.Errorf("%w: %q: %v", ErrInvalidIPAddress, ipStr, err)
		}
	}

	manager := &ConfigMapVMPoolManager{
		client: client,
		config: config,
	}

	return manager, nil
}

// getCurrentNodeName attempts to determine the current node name using multiple strategies
func getCurrentNodeName() (string, error) {
	// Strategy 1: Try environment variable first (set by CAA deployment)
	if nodeName := os.Getenv(nodeNameEnvVar); nodeName != "" {
		nodeName = strings.TrimSpace(nodeName)
		logger.Printf("Node name detected from environment: %s", nodeName)
		return nodeName, nil
	}

	// Strategy 2: Try downward API volume mount
	if data, err := os.ReadFile(nodeNameFile); err == nil {
		nodeName := strings.TrimSpace(string(data))
		if nodeName != "" {
			logger.Printf("Node name detected from downward API: %s", nodeName)
			return nodeName, nil
		}
	}

	// Strategy 3: Fallback to hostname (less reliable but better than nothing)
	if data, err := os.ReadFile(hostnameFile); err == nil {
		nodeName := strings.TrimSpace(string(data))
		if nodeName != "" {
			logger.Printf("Node name detected from hostname: %s", nodeName)
			return nodeName, nil
		}
	}

	return "", fmt.Errorf("%w: tried env var %s, file %s, and %s",
		ErrNodeNameDetection, nodeNameEnvVar, nodeNameFile, hostnameFile)
}

// marshalStateForConfigMap formats the state as indented JSON suitable for ConfigMap storage
func (cm *ConfigMapVMPoolManager) marshalStateForConfigMap(state *IPAllocationState) (string, error) {
	// Use 2-space indentation for clean formatting
	formattedJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal state with formatting: %w", err)
	}

	return string(formattedJSON), nil
}

// selectIPIndex uses hash-based distribution to select an IP index from available IPs
// This reduces conflicts when multiple CAA instances try to allocate simultaneously
func (cm *ConfigMapVMPoolManager) selectIPIndex(availableIPs []string, allocationID string) int {
	if len(availableIPs) <= 1 {
		return 0
	}

	// Use MD5 hash of allocation ID for deterministic but distributed selection
	// Different allocation IDs will consistently map to different indices
	hash := md5.Sum([]byte(allocationID))
	seed := binary.BigEndian.Uint32(hash[:4])

	selectedIndex := int(seed) % len(availableIPs)
	logger.Printf("Hash-based IP selection: allocationID=%s, hash=%x, index=%d/%d",
		allocationID, hash[:4], selectedIndex, len(availableIPs))

	return selectedIndex
}

// checkVMReadiness verifies that a VM is ready by checking network connectivity
func (cm *ConfigMapVMPoolManager) checkVMReadiness(ctx context.Context, ipStr string) error {
	logger.Printf("Checking VM readiness for IP %s", ipStr)

	// Use retry package for consistent retry behavior
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		// Retry on any connection error
		return true
	}, func() error {
		// Create a connection with timeout to check if VM is responding
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ipStr, "22"), 2*time.Second)
		if err != nil {
			logger.Printf("VM %s not ready: %v", ipStr, err)
			return err
		}
		conn.Close()
		logger.Printf("VM %s is ready", ipStr)
		return nil
	})
}

// AllocateIP allocates an IP from the global pool
func (cm *ConfigMapVMPoolManager) AllocateIP(ctx context.Context, allocationID string, podName string) (netip.Addr, error) {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	// Direct allocation - retry logic is handled inside updateState
	allocatedIP, err := cm.doAllocateIP(ctx, allocationID, podName)
	if err != nil {
		return netip.Addr{}, err
	}

	logger.Printf("Successfully allocated IP %s to allocation ID %s", allocatedIP.String(), allocationID)
	return allocatedIP, nil
}

// doAllocateIP performs the actual allocation with optimistic locking and smart IP selection
func (cm *ConfigMapVMPoolManager) doAllocateIP(ctx context.Context, allocationID string, podName string) (netip.Addr, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get current state
	state, _, err := cm.getCurrentState(ctx)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %w", ErrRetrievingPoolState, err)
	}

	// Check if already allocated
	if allocation, exists := state.AllocatedIPs[allocationID]; exists {
		ip, parseErr := netip.ParseAddr(allocation.IP)
		if parseErr != nil {
			return netip.Addr{}, fmt.Errorf("%w: %s: %w", ErrInvalidAllocatedIP, allocation.IP, parseErr)
		}
		logger.Printf("IP %s already allocated to allocation ID %s", allocation.IP, allocationID)
		return ip, nil
	}

	// Check if any IPs are available
	if len(state.AvailableIPs) == 0 {
		return netip.Addr{}, ErrNoAvailableIPs
	}

	// IP selection: use hash-based distribution to reduce conflicts
	selectedIndex := cm.selectIPIndex(state.AvailableIPs, allocationID)
	ipStr := state.AvailableIPs[selectedIndex]
	logger.Printf("Selected IP %s (index %d of %d) for allocation %s",
		ipStr, selectedIndex, len(state.AvailableIPs), allocationID)

	// Verify VM is ready before committing to allocation (skip in test mode)
	if !cm.config.SkipVMReadiness {
		if err := cm.checkVMReadiness(ctx, ipStr); err != nil {
			logger.Printf("VM %s failed readiness check. Can't be allocated: %v", ipStr, err)
			return netip.Addr{}, fmt.Errorf("%w: %s: %w", ErrInvalidAllocatedIP, ipStr, err)
		}
	} else {
		logger.Printf("Skipping VM readiness check for IP %s (test mode)", ipStr)
	}

	// Remove selected IP from available pool
	state.AvailableIPs = append(
		state.AvailableIPs[:selectedIndex],
		state.AvailableIPs[selectedIndex+1:]...)

	// Get current node name
	nodeName, err := getCurrentNodeName()
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %w", ErrNodeNameDetection, err)
	}

	// Add to allocated IPs
	state.AllocatedIPs[allocationID] = IPAllocation{
		AllocationID: allocationID,
		IP:           ipStr,
		NodeName:     nodeName,
		PodName:      podName,
		AllocatedAt:  metav1.Now(),
	}

	state.LastUpdated = metav1.Now()
	state.Version = state.Version + 1

	// Update ConfigMap - retry logic handled internally in updateState
	if err := cm.updateState(ctx, state); err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %w", ErrConflict, err)
	}

	// Convert to netip.Addr before returning
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %s: %w", ErrInvalidAllocatedIP, ipStr, err)
	}

	logger.Printf("Successfully allocated IP %s to allocation %s on node %s",
		ip.String(), allocationID, nodeName)

	return ip, nil
}

// DeallocateIP returns an IP to the global pool by allocation ID
func (cm *ConfigMapVMPoolManager) DeallocateIP(ctx context.Context, allocationID string) error {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	// Direct deallocation - retry logic is handled inside updateState
	if err := cm.doDeallocateIP(ctx, allocationID); err != nil {
		return err
	}

	logger.Printf("Successfully deallocated IP for allocation ID %s", allocationID)
	return nil
}

// doDeallocateIP performs the actual deallocation with optimistic locking
func (cm *ConfigMapVMPoolManager) doDeallocateIP(ctx context.Context, allocationID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get current state
	state, _, err := cm.getCurrentState(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRetrievingPoolState, err)
	}

	// Find allocation
	allocation, exists := state.AllocatedIPs[allocationID]
	if !exists {
		logger.Printf("allocation ID %s not found", allocationID)
		return nil
	}

	// Return IP to available pool
	state.AvailableIPs = append(state.AvailableIPs, allocation.IP)
	delete(state.AllocatedIPs, allocationID)

	state.LastUpdated = metav1.Now()
	state.Version = state.Version + 1

	// Update ConfigMap - retry logic handled internally in updateState
	if err := cm.updateState(ctx, state); err != nil {
		return fmt.Errorf("%w: %w", ErrUpdatingPoolState, err)
	}

	logger.Printf("Successfully deallocated IP %s", allocation.IP)
	return nil
}

// GetIPfromAllocationID returns the IP allocated to a specific allocation ID
func (cm *ConfigMapVMPoolManager) GetIPfromAllocationID(ctx context.Context, allocationID string) (netip.Addr, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	state, _, err := cm.getCurrentState(ctx)
	if err != nil {
		return netip.Addr{}, false, fmt.Errorf("%w: %w", ErrRetrievingPoolState, err)
	}

	allocation, exists := state.AllocatedIPs[allocationID]
	if !exists {
		return netip.Addr{}, false, nil
	}

	ip, err := netip.ParseAddr(allocation.IP)
	if err != nil {
		return netip.Addr{}, false, fmt.Errorf("%w: %s: %w", ErrInvalidAllocatedIP, allocation.IP, err)
	}

	return ip, true, nil
}

// GetAllocationIDfromIP returns the allocation ID for a given IP address
func (cm *ConfigMapVMPoolManager) GetAllocationIDfromIP(ctx context.Context, ip netip.Addr) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	state, _, err := cm.getCurrentState(ctx)
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", ErrRetrievingPoolState, err)
	}

	ipStr := ip.String()
	for allocationID, allocation := range state.AllocatedIPs {
		if allocation.IP == ipStr {
			return allocationID, true, nil
		}
	}

	return "", false, nil
}

// GetPoolStatus returns current pool statistics
func (cm *ConfigMapVMPoolManager) GetPoolStatus(ctx context.Context) (total, available, inUse int, err error) {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	state, _, err := cm.getCurrentState(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("%w: %w", ErrRetrievingPoolState, err)
	}

	available = len(state.AvailableIPs)
	inUse = len(state.AllocatedIPs)
	total = available + inUse

	return total, available, inUse, nil
}

// ListAllocatedIPs returns all currently allocated IPs
func (cm *ConfigMapVMPoolManager) ListAllocatedIPs(ctx context.Context) (map[string]IPAllocation, error) {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	state, _, err := cm.getCurrentState(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRetrievingPoolState, err)
	}

	// Return a copy to prevent external modifications
	result := make(map[string]IPAllocation, len(state.AllocatedIPs))
	for id, allocation := range state.AllocatedIPs {
		result[id] = allocation
	}

	return result, nil
}

// getCurrentState retrieves the current allocation state from ConfigMap with ResourceVersion
func (cm *ConfigMapVMPoolManager) getCurrentState(ctx context.Context) (*IPAllocationState, string, error) {
	configMap, err := cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Get(
		ctx, cm.config.ConfigMapName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		// Initialize empty state
		return cm.initializeEmptyState(), "", nil
	}

	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrRetrievingConfigMap, err)
	}

	stateData, exists := configMap.Data[stateDataKey]
	if !exists {
		// Initialize empty state
		return cm.initializeEmptyState(), "", nil
	}

	var state IPAllocationState
	if err := json.Unmarshal([]byte(stateData), &state); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	// Return ResourceVersion for true optimistic locking
	return &state, configMap.ResourceVersion, nil
}

// updateState updates the allocation state in ConfigMap with proper optimistic locking
// This method handles all retry logic internally and is the single point for ConfigMap updates
func (cm *ConfigMapVMPoolManager) updateState(ctx context.Context, state *IPAllocationState) error {
	// Use formatted JSON for better readability
	formattedState, err := cm.marshalStateForConfigMap(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state data: %w", err)
	}

	// Use RetryOnConflict for the entire get-modify-update loop.
	// This also gracefully handles the case where the ConfigMap doesn't exist yet.
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// 1. READ: Get the latest version of the ConfigMap.
		configMap, err := cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Get(
			ctx, cm.config.ConfigMapName, metav1.GetOptions{})

		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist, so we create it.
			newConfigMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cm.config.ConfigMapName,
					Namespace: cm.config.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":      "cloud-api-adaptor",
						"app.kubernetes.io/component": "byom-ip-pool-state",
					},
				},
				Data: map[string]string{stateDataKey: formattedState},
			}
			_, createErr := cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Create(ctx, newConfigMap, metav1.CreateOptions{})
			if createErr == nil {
				logger.Printf("Created new ConfigMap %s with initial state", cm.config.ConfigMapName)
			} else {
				logger.Printf("Failed to create ConfigMap %s: %v", cm.config.ConfigMapName, createErr)
			}
			// If creation fails with "already exists", the retry loop will handle it.
			return createErr
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrRetrievingConfigMap, err)
		}

		// 2. MODIFY: Apply changes to the fetched object.
		// Make a copy to avoid modifying the cache.
		configMapToUpdate := configMap.DeepCopy()
		if configMapToUpdate.Data == nil {
			configMapToUpdate.Data = make(map[string]string)
		}
		configMapToUpdate.Data[stateDataKey] = formattedState

		// 3. WRITE: Attempt the update.
		// Kubernetes API server will reject this if configMapToUpdate.ResourceVersion
		// is stale. RetryOnConflict will then re-execute this whole function.
		_, updateErr := cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Update(ctx, configMapToUpdate, metav1.UpdateOptions{})
		if updateErr == nil {
			logger.Printf("Successfully updated ConfigMap %s with new state (version %d)",
				cm.config.ConfigMapName, state.Version)
		} else {
			logger.Printf("Failed to update ConfigMap %s: %v", cm.config.ConfigMapName, updateErr)
		}
		return updateErr
	})
}
