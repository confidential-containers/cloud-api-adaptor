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
Failure case: If for any reason cloud-api-adaptor doesn’t honor the delete request or it fails to perform deletion, the finalizer is not removed. Hence, when PeerPod controller gets a delete event for the owned PeerPod object by the GC and it still has the finalizer, it will comprehend that it needs to perform the deletion of pod VM resource by itself, based on the PeerPod CR fields.

## Getting Started
You’ll need a Kubernetes cluster on a [supported provider](../README.md#supported-providers) to run against (e.g. you can use [Libvirt for development](../libvirt)).
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
Make sure to [install cloud-api-adaptor](../install/README.md) first
```sh
make deploy
```

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
For any changes in the CRD/controller make sure it doesn't break the k8s api calls from the [cloud-api-adaptor](../) and adapt it if needed.

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
