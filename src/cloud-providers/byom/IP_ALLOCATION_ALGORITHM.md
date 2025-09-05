# BYOM IP Allocation Algorithm Documentation

## Overview

The BYOM (Bring Your Own Machine) provider implements a distributed IP allocation system that manages a global pool of pre-created VM IP addresses across multiple Cloud API Adaptor (CAA) instances. This document describes the algorithm, state management, and key functions.

## Architecture

### Components

1. **ConfigMapVMPoolManager**: Main component managing IP allocation state
2. **Kubernetes ConfigMap**: Persistent storage for global allocation state
3. **Optimistic Locking**: Conflict resolution using Kubernetes ResourceVersion
4. **Hash-based Distribution**: Reduces allocation conflicts between CAA instances

### State Structure

```go
type IPAllocationState struct {
    AllocatedIPs map[string]IPAllocation  // allocationID -> allocation details
    AvailableIPs []string                 // list of free IP addresses
    LastUpdated  metav1.Time             // timestamp of last update
    Version      int64                   // monotonic version counter
}

type IPAllocation struct {
    AllocationID string      // unique identifier for allocation
    IP           string      // allocated IP address
    NodeName     string      // CAA node that made the allocation
    PodName      string      // pod name for tracking
    PodNamespace string      // pod namespace for tracking
    AllocatedAt  metav1.Time // allocation timestamp
}
```

## Core Algorithm

### 1. IP Allocation Process

#### Hash-based IP Selection

```
function selectIPIndex(availableIPs, allocationID):
    if len(availableIPs) <= 1:
        return 0
    
    hash = MD5(allocationID)
    seed = first_4_bytes_as_uint32(hash)
    return seed % len(availableIPs)
```

**Purpose**: Distribute allocation attempts across different IPs to reduce conflicts when multiple CAA instances allocate simultaneously.

**Benefits**:
- Same allocationID consistently maps to same index (if available)
- Different allocationIDs spread across different indices
- Reduces "thundering herd" problems

#### Allocation Flow

```
function AllocateIP(ctx, allocationID, podName, podNamespace):
    retry with optimistic locking:
        state, resourceVersion = getCurrentState(ctx)
        
        // Idempotency check
        if allocation exists in state.AllocatedIPs[allocationID]:
            return existing allocation
        
        if len(state.AvailableIPs) == 0:
            return error "no available IPs"
        
        // Smart selection to reduce conflicts
        selectedIndex = selectIPIndex(state.AvailableIPs, allocationID)
        selectedIP = state.AvailableIPs[selectedIndex]
        
        // Remove from available, add to allocated
        state.AvailableIPs.remove(selectedIndex)
        state.AllocatedIPs[allocationID] = IPAllocation{
            AllocationID: allocationID,
            IP: selectedIP,
            NodeName: getCurrentNodeName(),
            PodName: podName,
            PodNamespace: podNamespace,
            AllocatedAt: now()
        }
        
        state.Version++
        
        // Atomic update with conflict detection
        updateState(ctx, state, resourceVersion)
```

### 2. Optimistic Locking Implementation

#### Conflict Detection

```
function updateState(ctx, newState, expectedResourceVersion):
    currentConfigMap = kubernetes.get(configMapName)
    
    // Critical: Check for concurrent modifications
    if currentConfigMap.ResourceVersion != expectedResourceVersion:
        return ConflictError
    
    currentConfigMap.data = serialize(newState)
    kubernetes.update(currentConfigMap)  // This may also return 409 Conflict
```

#### Retry Logic

```
function AllocateIPWithRetry(ctx, allocationID, podName, podNamespace):
    return retry.RetryOnConflict(retry.DefaultBackoff, func():
        return doAllocateIP(ctx, allocationID, podName, podNamespace)
    )
```

**Backoff Strategy**: Kubernetes client-go default exponential backoff
- Initial delay: 500ms
- Factor: 1.5
- Jitter: true
- Max retries: 5

### 3. State Recovery Algorithm

State recovery handles CAA restarts and ensures VM consistency.

#### Recovery Flow

```
function RecoverState(ctx):
    currentNode = getCurrentNodeName()
    
    state, resourceVersion = getCurrentState(ctx)
    if state == nil:
        return initializeEmptyState(ctx)
    
    // Release allocations from restarting node (VMs are now invalid)
    releaseNodeAllocations(ctx, state, currentNode, resourceVersion)
    
    // Validate and repair any inconsistencies
    validateAndRepairState(ctx)
```

#### VM Cleanup Integration

```
function releaseNodeAllocations(ctx, state, nodeName, resourceVersion, vmCleanupFunc):
    nodeIPs = []
    for allocation in state.AllocatedIPs:
        if allocation.NodeName == nodeName:
            nodeIPs.append(allocation.IP)
    
    // Send reboot files to VMs FIRST (critical for consistency)
    cleanupResults = {}
    if vmCleanupFunc != nil:
        for ip in nodeIPs:
            cleanupResults[ip] = vmCleanupFunc(ip)
        
        // Wait for VMs to process reboot
        wait(5 seconds)
    
    // Only release IPs with successful cleanup
    successfulIPs = []
    failedIPs = []
    for ip in nodeIPs:
        if cleanupResults[ip] == nil:  // success or no cleanup function
            successfulIPs.append(ip)
        else:
            failedIPs.append(ip)
            log("NOT releasing IP due to failed cleanup")
    
    // Update state: release successful IPs, keep failed ones allocated
    state.AvailableIPs.extend(successfulIPs)
    state.AllocatedIPs = removeByIPs(state.AllocatedIPs, successfulIPs)
    
    updateState(ctx, state, resourceVersion)
```

## Conflict Resolution

### 1. Race Condition Scenarios

#### Scenario A: Multiple CAA instances allocate simultaneously
```
Timeline:
CAA-1: getCurrentState() -> AvailableIPs: ["IP1", "IP2", "IP3"]
CAA-2: getCurrentState() -> AvailableIPs: ["IP1", "IP2", "IP3"]
CAA-1: hash(alloc-1) % 3 = 1 -> selects "IP2"
CAA-2: hash(alloc-2) % 3 = 0 -> selects "IP1"  // Different IP!
CAA-1: updateState() -> SUCCESS
CAA-2: updateState() -> SUCCESS
Result: Both succeed with different IPs
```

#### Scenario B: Hash collision (rare)
```
Timeline:
CAA-1: hash(alloc-1) % 3 = 1 -> selects "IP2"
CAA-2: hash(alloc-2) % 3 = 1 -> selects "IP2"  // Same IP!
CAA-1: updateState(resourceVersion="v1") -> SUCCESS
CAA-2: updateState(resourceVersion="v1") -> CONFLICT (resource version changed)
CAA-2: retry -> getCurrentState() -> AvailableIPs: ["IP1", "IP3"]  // IP2 gone
CAA-2: hash(alloc-2) % 2 = 0 -> selects "IP1"
CAA-2: updateState() -> SUCCESS
Result: CAA-2 retries with different IP, both succeed
```

### 2. Error Handling

#### Allocation Errors
- **Pool Exhausted**: `"no available IPs in pool"` - Expected when demand > supply
- **Conflict**: Handled by retry mechanism - indicates concurrent access
- **Invalid State**: Triggers state repair and validation

#### Recovery Errors
- **VM Cleanup Failure**: IP remains allocated to prevent VM state inconsistency
- **State Corruption**: Automatic repair removes invalid entries
- **Network Timeout**: Configurable timeouts with proper context cancellation

## Performance Characteristics

### Time Complexity
- **Allocation**: O(1) for IP selection, O(n) for ConfigMap update serialization
- **Deallocation**: O(1) for removal, O(n) for ConfigMap update serialization
- **State Recovery**: O(n) where n = number of allocated IPs

### Space Complexity
- **Memory**: O(n) where n = total IPs in pool
- **Storage**: O(n) in Kubernetes ConfigMap

### Scalability
- **Nodes**: Linear scalability - each node operates independently
- **IPs**: Limited by ConfigMap size (typically ~1MB, supports thousands of IPs)
- **Conflicts**: Exponentially reduced by hash-based distribution

## Configuration

### Key Parameters

```go
type GlobalVMPoolConfig struct {
    Namespace        string        // Kubernetes namespace for ConfigMap
    ConfigMapName    string        // Name of ConfigMap storing state
    PoolIPs          []string      // List of available IP addresses
    OperationTimeout time.Duration // Timeout for Kubernetes operations
    MaxRetries       int           // Maximum retry attempts
    RetryInterval    time.Duration // Base retry interval
}
```

### Tuning Guidelines

1. **OperationTimeout**: 
   - Default: 30 seconds
   - Increase for large clusters or slow storage
   
2. **VM Cleanup Wait**:
   - Default: 5 seconds
   - Adjust based on VM reboot characteristics
   
3. **Pool Size**:
   - Keep below 1000 IPs per ConfigMap for optimal performance
   - Use multiple pools for larger deployments

## Security Considerations

1. **RBAC**: ConfigMap access requires appropriate Kubernetes permissions
2. **State Validation**: All IP addresses validated against configured pool
3. **Input Sanitization**: Allocation IDs and pod names validated
4. **Audit Trail**: All operations logged with timestamps and node information

## Monitoring and Observability

### Key Metrics
- Pool utilization rate
- Allocation success/failure rates
- Conflict frequency
- Average allocation time
- VM cleanup success rate

### Log Messages
- Allocation attempts and results
- Conflict detection and resolution
- State recovery operations
- VM cleanup status

### Debugging
- ConfigMap inspection: `kubectl get configmap <name> -o yaml`
- Pool status: Available through `GetPoolStatus()` API
- Allocation history: Tracked in IPAllocation timestamps

## Future Enhancements

1. **Metrics Collection**: Prometheus metrics for monitoring
2. **Dynamic Pool Sizing**: Automatic scaling based on demand
3. **Multi-Pool Support**: Distribute load across multiple ConfigMaps
4. **Improved Hash Distribution**: Consider consistent hashing for better distribution
5. **Configurable VM Cleanup Wait**: Make wait duration configurable per deployment

## Troubleshooting

### Common Issues

1. **High Conflict Rate**
   - **Cause**: Poor hash distribution or insufficient IP pool
   - **Solution**: Increase pool size or investigate allocation patterns

2. **Pool Exhaustion**
   - **Cause**: More pods than available IPs
   - **Solution**: Add more IPs to pool or implement pod scheduling constraints

3. **State Inconsistency**
   - **Cause**: Concurrent modifications or partial failures
   - **Solution**: Automatic state repair runs on startup

4. **VM Cleanup Failures**
   - **Cause**: Network issues or VM unavailability
   - **Solution**: IPs remain allocated; manual intervention may be required

### Diagnostic Commands

```bash
# Check pool status
kubectl get configmap <configmap-name> -n <namespace> -o yaml

# Monitor CAA logs
kubectl logs -f <caa-pod> | grep "IP allocation"

# Verify IP pool configuration
kubectl describe configmap <configmap-name> -n <namespace>
```
