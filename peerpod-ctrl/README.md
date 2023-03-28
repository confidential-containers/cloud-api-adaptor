# peerpod-ctrl
The PeerPod CR is used to track peer-pods cloud-provider resources, it specifies the InstanceID and its CloudProvider and it’s owned by the Pod that represents the PeerPod
The PeerPod controller is watching PeerPod events and deleting hanging resources that were not deleted by the cloud-api-adaptor at Pod deletion time.

## Description
### Creation time:
With every successful peer-pod VM creation cloud-api-adaptor will create also a PeePod CR (predefined by the operator) which contains the cloud provider and its VM instance id.

### Owner references:
The PeerPod CR is owned by the original Pod object that represents the peer-pod, upon pod deletion background cascading deletion getting into action which means the Pod will be deleted first and the GC will handle the owned PeerPod CR later.

### Deletion time:
* Normal case: When remote hypervisor will get the stopVM request (upon pod deletion) it will delete the podVM instance and if successful it will remove the finalizer attached to the owned PeerPod object so it can then be cleaned by the GC
* Failure case: In case for some reason cloud-api-adaptor doesn’t honor the delete request of it fails to delete it, the finalizer is not removed and then when peer-pods controller will recognize deletion that the GC is trying to perform for the owned PeerPod object and that it still has the finalizer it will comprehend it needs to preform the deletion of podVM resource based on the PeerPod CR fields

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
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
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright Confidential Containers Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

