# peerpod-ctrl
The PeerPod CR is used to track the cloud provider resources for a (peer)Pod; it requires the cloud InstanceID and the CloudProvider. PeerPod objects are owned by the matching Pod object.
The PeerPod controller is watching PeerPod events and deleting dangling resources that were not deleted by the cloud-api-adaptor at Pod deletion time.

## Description
### Creation time:
With every successful VM creation for a Pod, cloud-api-adaptor will create a PeePod CR (predefined by the operator) which contains the VM instance id and cloud provider.

### Owner references:
The PeerPod CR is owned by the original Pod object. Upon Pod deletion [background cascading deletion](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#background-deletion) gets into action and hence the Pod will be deleted first, followed by GC handling the owned PeerPod CR.

### Deletion time:
Normal case: When remote hypervisor will get the stopVM request (upon Pod deletion) it will delete the pod VM instance and if it succeeds it will remove the finalizer attached to the owned PeerPod object so it can then be cleaned by the GC.

Failure case: If for any reason cloud-api-adaptor doesn't honor the delete request or it fails to perform deletion, the finalizer is not removed. Hence, when PeerPod controller gets a delete event for the owned PeerPod object by the GC and it still has the finalizer, it will comprehend that it needs to perform the deletion of pod VM resource by itself, based on the PeerPod CR fields.

### Orphan VM garbage collection
If the cloud-api-adaptor crashes after creating a VM but before the corresponding PeerPod CR is written, the VM becomes an orphan with no Kubernetes object tracking it. The peerpod-ctrl includes a periodic garbage collector that detects and deletes these orphan VMs.

**How it works:**
1. At startup, the cloud-api-adaptor tags every VM it creates with a `caa-cluster-uid` tag containing the `kube-system` namespace UID, uniquely identifying the cluster.
2. The garbage collector periodically calls `ListInstances` on the cloud provider to discover all VMs tagged with this cluster's UID.
3. It compares the discovered VMs against existing PeerPod CRs. Any VM without a matching PeerPod CR is considered an orphan and deleted.

**Grace period:** To avoid deleting VMs that were just created but whose PeerPod CR has not yet been written, the GC applies a 10-minute grace period. Instances are only deleted after the GC has observed them as orphan candidates for at least 10 minutes across consecutive cycles (using only the controller's local clock, avoiding any cross-clock dependency with the cloud provider). The 10-minute default provides a safety margin for large GPU instances that can take 5+ minutes to boot.

**Timing:** An orphan is deleted on the first GC cycle after both (a) it has been discovered and (b) the grace period has elapsed since discovery. With defaults (`GC_INTERVAL=30m`, grace period 10m), deletion occurs on the cycle after first discovery (~30m), since 30m > 10m. Worst-case time from VM creation to deletion is approximately 2x `GC_INTERVAL` (one interval to discover, one to delete).

**Configuration** via the `peer-pods-cm` ConfigMap:

| Key | Default | Hot-reloadable | Description |
|---|---|---|---|
| `ENABLE_GC` | `true` | No (requires pod restart) | Set to `false` to disable orphan VM garbage collection |
| `GC_INTERVAL` | `30m` | Yes | How often the GC runs (Go duration string, must be > 0) |
| `GC_GRACE_PERIOD` | `10m` | Yes | How long an orphan candidate must be observed before deletion (Go duration string, must be > 0) |

**Note:** Changing `GC_GRACE_PERIOD` at runtime resets the firstSeen tracking for all orphan candidates, so they must be re-observed for the new grace period before deletion.

**Provider support:** The garbage collector requires the cloud provider to implement the `InstanceLister` interface. Currently supported: AWS. Providers that do not implement this interface are gracefully skipped.

## Getting Started
You'll need a Kubernetes cluster on a [supported provider](../../README.md#supported-providers) to run against (e.g. you can use [Libvirt for development](../cloud-api-adaptor/libvirt)).
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
Make sure to [install cloud-api-adaptor](../cloud-api-adaptor/install/README.md) first
```sh
make deploy
```
**Note:** alternatively you can deploy the peerpod-ctrl along with [cloud-api-adaptor installtion](../cloud-api-adaptor/install/README.md) by setting `RESOURCE_CTRL=true`

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing
For any changes in the CRD/controller make sure it doesn't break the k8s api calls from the [cloud-api-adaptor](../cloud-api-adaptor) and adapt it if needed.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
#### Running custom build
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/peerpod-ctrl:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/peerpod-ctrl:tag
```

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
