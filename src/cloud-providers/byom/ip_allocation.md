# BYOM IP Allocation

The BYOM (Bring Your Own Machine) provider manages a distributed pool of pre-created VM IP addresses across multiple CAA instances using Kubernetes ConfigMaps with optimistic locking.

## Components

- **ConfigMapVMPoolManager**: Main IP allocation manager (`configmap_vmpool.go`)
- **Kubernetes ConfigMap**: Persistent state storage
- **Optimistic Locking**: Conflict resolution using ResourceVersion
- **Hash-based Distribution**: Reduces allocation conflicts between CAA instances

## Configuration

**Default Namespace**: `confidential-containers-system`
**Default ConfigMap Name**: `byom-ip-pool-state`

## State Structure

```go
type IPAllocationState struct {
    AllocatedIPs map[string]IPAllocation `json:"allocatedIPs"`
    AvailableIPs []string                `json:"availableIPs"`
    LastUpdated  metav1.Time             `json:"lastUpdated"`
    Version      int64                   `json:"version"`
}

type IPAllocation struct {
    AllocationID string      `json:"allocationID"`
    IP           string      `json:"ip"`
    NodeName     string      `json:"nodeName"`
    PodName      string      `json:"podName"`
    PodNamespace string      `json:"podNamespace"`
    AllocatedAt  metav1.Time `json:"allocatedAt"`
}
```

## Hash-based IP Selection

Implemented in `configmap_vmpool.go`:

```go
func (cm *ConfigMapVMPoolManager) selectIPIndex(availableIPs []string, allocationID string) int {
    if len(availableIPs) <= 1 {
        return 0
    }
    hash := md5.Sum([]byte(allocationID))
    seed := binary.BigEndian.Uint32(hash[:4])
    return int(seed) % len(availableIPs)
}
```

**Benefits**: Same allocationID maps to same index; different IDs spread across indices, reducing conflicts.

## Optimistic Locking

Implemented in `configmap_vmpool.go`:

1. **Conflict Detection**: Compare expected vs actual ResourceVersion
2. **Retry Strategy**: Uses `retry.RetryOnConflict(retry.DefaultBackoff)`
   - Initial delay: 500ms, Factor: 1.5, Jitter: true, Max retries: 5
3. **Atomic Updates**: Kubernetes handles final conflict detection on update

## State Recovery

Implemented in `state_recovery.go`. On CAA restart:

1. **Node Detection**: Uses `NODE_NAME` env, `/etc/podinfo/nodename`, or `/etc/hostname`
2. **Cleanup Integration**: Sends reboot signals to VMs before releasing IPs
3. **Safety**: IPs with failed cleanup remain allocated to prevent inconsistency
4. **Recovery Interface**: `RecoverState(ctx)` method in `GlobalVMPoolManager`

## Conflict Resolution

**Hash Distribution**: Different allocation IDs typically select different IPs, reducing conflicts.

**Collision Handling**: When multiple CAA instances select the same IP:
1. First update succeeds
2. Subsequent updates get ResourceVersion conflict
3. Retry with fresh state (hash selects from remaining IPs)

**Viewing current state**:

```sh
kubectl get cm byom-ip-pool-state -n confidential-containers-system -o yaml
```
