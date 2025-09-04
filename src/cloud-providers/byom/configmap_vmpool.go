// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		config = DefaultGlobalVMPoolConfig()
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
		return netip.Addr{}, fmt.Errorf("%w: %w", ErrAllocationRetryExhausted, retryErr)
	}

	// Note: PeerPod CR will automatically contain the IP in spec.instanceID when created
	// No additional observability update needed here

	logger.Printf("Successfully allocated IP %s to allocation ID %s", allocatedIP.String(), allocationID)
	return allocatedIP, nil
}

// doAllocateIP performs the actual allocation with optimistic locking and smart IP selection
func (cm *ConfigMapVMPoolManager) doAllocateIP(ctx context.Context, allocationID string, podName, podNamespace string) (netip.Addr, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get current state with ResourceVersion for optimistic locking
	state, resourceVersion, err := cm.getCurrentState(ctx)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %w", ErrRetrievingPoolState, err)
	}

	// Check if already allocated (idempotent operation)
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

	// Smart IP selection: use hash-based distribution to reduce conflicts
	selectedIndex := cm.selectIPIndex(state.AvailableIPs, allocationID)
	ipStr := state.AvailableIPs[selectedIndex]
	logger.Printf("Selected IP %s (index %d of %d) for allocation %s",
		ipStr, selectedIndex, len(state.AvailableIPs), allocationID)

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
		PodNamespace: podNamespace,
		AllocatedAt:  metav1.Now(),
	}

	state.LastUpdated = metav1.Now()
	state.Version = state.Version + 1

	// Update ConfigMap with ResourceVersion check for conflict detection
	if err := cm.updateState(ctx, state, resourceVersion); err != nil {
		if errors.IsConflict(err) {
			logger.Printf("Conflict detected for allocation %s, will retry with fresh state", allocationID)
			return netip.Addr{}, err // RetryOnConflict will retry this
		}
		return netip.Addr{}, fmt.Errorf("%w: %w", ErrConflict, err)
	}

	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %s: %w", ErrInvalidAllocatedIP, ipStr, err)
	}

	logger.Printf("Successfully allocated IP %s to allocation %s on node %s",
		ipStr, allocationID, nodeName)
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
		return fmt.Errorf("%w: %w", ErrDeallocationRetryExhausted, retryErr)
	}

	logger.Printf("Successfully deallocated IP for allocation ID %s", allocationID)
	return nil
}

// doDeallocateIP performs the actual deallocation with optimistic locking
func (cm *ConfigMapVMPoolManager) doDeallocateIP(ctx context.Context, allocationID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get current state with ResourceVersion
	state, resourceVersion, err := cm.getCurrentState(ctx)
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

	// Update ConfigMap atomically with conflict detection
	if err := cm.updateState(ctx, state, resourceVersion); err != nil {
		if errors.IsConflict(err) {
			logger.Printf("Conflict detected for deallocation %s, will retry", allocationID)
			return err // RetryOnConflict will retry this
		}
		return fmt.Errorf("%w: %w", ErrUpdatingPoolState, err)
	}

	logger.Printf("Successfully deallocated IP %s for allocation %s", allocation.IP, allocationID)
	return nil
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
		logger.Printf("IP %s not found in allocated pool", ip.String())
		return nil
	}

	return cm.DeallocateIP(ctx, allocationID)
}

// GetAllocatedIP returns the IP allocated to a specific allocation ID
func (cm *ConfigMapVMPoolManager) GetAllocatedIP(ctx context.Context, allocationID string) (netip.Addr, bool, error) {
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
		return nil, "", fmt.Errorf("%w: %w", ErrUpdatingConfigMap, err)
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
func (cm *ConfigMapVMPoolManager) updateState(ctx context.Context, state *IPAllocationState, expectedResourceVersion string) error {
	// Use formatted JSON for better readability
	formattedState, err := cm.marshalStateForConfigMap(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state data: %w", err)
	}

	// Get current ConfigMap to check ResourceVersion
	configMap, err := cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Get(
		ctx, cm.config.ConfigMapName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		// Create new ConfigMap - first time initialization
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
			return fmt.Errorf("%w: %w", ErrUpdatingConfigMap, err)
		}
		logger.Printf("Created new ConfigMap %s with initial state", cm.config.ConfigMapName)
		return nil
	}

	if err != nil {
		return fmt.Errorf("%w: %w", ErrRetrievingConfigMap, err)
	}

	// Check for conflicts
	if expectedResourceVersion != "" && configMap.ResourceVersion != expectedResourceVersion {
		logger.Printf("ResourceVersion conflict: expected %s, got %s",
			expectedResourceVersion, configMap.ResourceVersion)
		return errors.NewConflict(
			schema.GroupResource{Resource: "configmaps"},
			cm.config.ConfigMapName,
			fmt.Errorf("ResourceVersion conflict: expected %s, got %s",
				expectedResourceVersion, configMap.ResourceVersion))
	}

	// Update existing ConfigMap with conflict detection
	configMap.Data[stateDataKey] = formattedState

	_, err = cm.client.CoreV1().ConfigMaps(cm.config.Namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		// This will return 409 Conflict if another process updated it
		return fmt.Errorf("%w: %w", ErrUpdatingConfigMap, err)
	}

	logger.Printf("Successfully updated ConfigMap %s with new state (version %d)",
		cm.config.ConfigMapName, state.Version)
	return nil
}
