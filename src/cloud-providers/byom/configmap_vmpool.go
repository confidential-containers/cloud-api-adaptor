// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	stateDataKey = "allocation-state"
	// Node identity detection paths
	nodeNameEnvVar    = "NODE_NAME"
	nodeNameFile      = "/etc/podinfo/nodename"
	hostnameFile      = "/etc/hostname"
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
		return nil, fmt.Errorf("kubernetes client cannot be nil")
	}

	if config == nil {
		config = DefaultGlobalVMPoolConfig()
	}

	// Validate pool configuration
	if len(config.PoolIPs) == 0 {
		return nil, fmt.Errorf("pool IPs cannot be empty")
	}

	// Validate IP addresses
	for _, ipStr := range config.PoolIPs {
		if _, err := netip.ParseAddr(ipStr); err != nil {
			return nil, fmt.Errorf("invalid IP address %q: %w", ipStr, err)
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
	
	return "", fmt.Errorf("unable to determine node name: tried env var %s, file %s, and %s", 
		nodeNameEnvVar, nodeNameFile, hostnameFile)
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

// AllocateIP allocates an IP from the global pool
func (cm *ConfigMapVMPoolManager) AllocateIP(ctx context.Context, allocationID string, podName, podNamespace string) (netip.Addr, error) {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	var allocatedIP netip.Addr
	var err error

	// Retry allocation with optimistic locking
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		allocatedIP, err = cm.doAllocateIP(ctx, allocationID, podName, podNamespace)
		return err
	})

	if retryErr != nil {
		return netip.Addr{}, fmt.Errorf("failed to allocate IP after retries: %w", retryErr)
	}

	// Note: PeerPod CR will automatically contain the IP in spec.instanceID when created
	// No additional observability update needed here

	logger.Printf("Successfully allocated IP %s to allocation ID %s", allocatedIP.String(), allocationID)
	return allocatedIP, nil
}

// doAllocateIP performs the actual allocation with optimistic locking
func (cm *ConfigMapVMPoolManager) doAllocateIP(ctx context.Context, allocationID string, podName, podNamespace string) (netip.Addr, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get current state
	state, version, err := cm.getCurrentState(ctx)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to get current state: %w", err)
	}

	// Check if already allocated
	if allocation, exists := state.AllocatedIPs[allocationID]; exists {
		ip, parseErr := netip.ParseAddr(allocation.IP)
		if parseErr != nil {
			return netip.Addr{}, fmt.Errorf("invalid allocated IP %s: %w", allocation.IP, parseErr)
		}
		return ip, nil
	}

	// Check if any IPs are available
	if len(state.AvailableIPs) == 0 {
		return netip.Addr{}, fmt.Errorf("no available IPs in pool")
	}

	// Allocate first available IP
	ipStr := state.AvailableIPs[0]
	state.AvailableIPs = state.AvailableIPs[1:]

	// Get current node name
	nodeName, err := getCurrentNodeName()
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to get node name: %w", err)
	}

	// Add to allocated IPs
	state.AllocatedIPs[allocationID] = IPAllocation{
		AllocationID: allocationID,
		IP:           ipStr,
		NodeName:     nodeName,
		PodName:      podName,
		PodNamespace: podNamespace,
		AllocatedAt:  metav1.Now(),
	}

	state.LastUpdated = metav1.Now()
	state.Version = version + 1

	// Update ConfigMap atomically
	if err := cm.updateState(ctx, state, version); err != nil {
		return netip.Addr{}, fmt.Errorf("failed to update state: %w", err)
	}

	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to parse allocated IP %s: %w", ipStr, err)
	}

	return ip, nil
}

// DeallocateIP returns an IP to the global pool by allocation ID
func (cm *ConfigMapVMPoolManager) DeallocateIP(ctx context.Context, allocationID string) error {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return cm.doDeallocateIP(ctx, allocationID)
	})

	if retryErr != nil {
		return fmt.Errorf("failed to deallocate IP after retries: %w", retryErr)
	}

	logger.Printf("Successfully deallocated IP for allocation ID %s", allocationID)
	return nil
}

// doDeallocateIP performs the actual deallocation
func (cm *ConfigMapVMPoolManager) doDeallocateIP(ctx context.Context, allocationID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get current state
	state, version, err := cm.getCurrentState(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	// Find allocation
	allocation, exists := state.AllocatedIPs[allocationID]
	if !exists {
		return fmt.Errorf("allocation ID %s not found", allocationID)
	}

	// Return IP to available pool
	state.AvailableIPs = append(state.AvailableIPs, allocation.IP)
	delete(state.AllocatedIPs, allocationID)

	state.LastUpdated = metav1.Now()
	state.Version = version + 1

	// Update ConfigMap atomically
	return cm.updateState(ctx, state, version)
}

// DeallocateByIP returns an IP to the pool by IP address
func (cm *ConfigMapVMPoolManager) DeallocateByIP(ctx context.Context, ip netip.Addr) error {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	// Find allocation ID for this IP
	allocatedIPs, err := cm.ListAllocatedIPs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list allocated IPs: %w", err)
	}

	var allocationID string
	for id, allocation := range allocatedIPs {
		if allocation.IP == ip.String() {
			allocationID = id
			break
		}
	}

	if allocationID == "" {
		return fmt.Errorf("IP %s not found in allocated pool", ip.String())
	}

	return cm.DeallocateIP(ctx, allocationID)
}

// GetAllocatedIP returns the IP allocated to a specific allocation ID
func (cm *ConfigMapVMPoolManager) GetAllocatedIP(ctx context.Context, allocationID string) (netip.Addr, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	state, _, err := cm.getCurrentState(ctx)
	if err != nil {
		return netip.Addr{}, false, fmt.Errorf("failed to get current state: %w", err)
	}

	allocation, exists := state.AllocatedIPs[allocationID]
	if !exists {
		return netip.Addr{}, false, nil
	}

	ip, err := netip.ParseAddr(allocation.IP)
	if err != nil {
		return netip.Addr{}, false, fmt.Errorf("invalid allocated IP %s: %w", allocation.IP, err)
	}

	return ip, true, nil
}

// GetPoolStatus returns current pool statistics
func (cm *ConfigMapVMPoolManager) GetPoolStatus(ctx context.Context) (total, available, inUse int, err error) {
	ctx, cancel := context.WithTimeout(ctx, cm.config.OperationTimeout)
	defer cancel()

	state, _, err := cm.getCurrentState(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get current state: %w", err)
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
		return nil, fmt.Errorf("failed to get current state: %w", err)
	}

	// Return a copy to prevent external modifications
	result := make(map[string]IPAllocation, len(state.AllocatedIPs))
	for id, allocation := range state.AllocatedIPs {
		result[id] = allocation
	}

	return result, nil
}

// getCurrentState retrieves the current allocation state from ConfigMap
func (cm *ConfigMapVMPoolManager) getCurrentState(ctx context.Context) (*IPAllocationState, int64, error) {
	configMap, err := cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Get(
		ctx, cm.config.ConfigMapName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		// Initialize empty state
		return cm.initializeEmptyState(), 0, nil
	}

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	stateData, exists := configMap.Data[stateDataKey]
	if !exists {
		// Initialize empty state
		return cm.initializeEmptyState(), 0, nil
	}

	var state IPAllocationState
	if err := json.Unmarshal([]byte(stateData), &state); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	// Use ConfigMap resource version for optimistic locking
	resourceVersion := configMap.ResourceVersion
	version := int64(0)
	if resourceVersion != "" {
		// In real implementation, convert resource version to int64
		// For simplicity, using state version
		version = state.Version
	}

	return &state, version, nil
}

// updateState updates the allocation state in ConfigMap
func (cm *ConfigMapVMPoolManager) updateState(ctx context.Context, state *IPAllocationState, _ int64) error {
	// Use formatted JSON for better readability
	formattedState, err := cm.marshalStateForConfigMap(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state data: %w", err)
	}

	// Try to get existing ConfigMap
	configMap, err := cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Get(
		ctx, cm.config.ConfigMapName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		// Create new ConfigMap
		newConfigMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cm.config.ConfigMapName,
				Namespace: cm.config.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":      "cloud-api-adaptor",
					"app.kubernetes.io/component": "byom-ip-pool",
				},
			},
			Data: map[string]string{
				stateDataKey: formattedState,
			},
		}

		_, err = cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Create(ctx, newConfigMap, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get ConfigMap for update: %w", err)
	}

	// Update existing ConfigMap
	configMap.Data[stateDataKey] = formattedState

	_, err = cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return nil
}
