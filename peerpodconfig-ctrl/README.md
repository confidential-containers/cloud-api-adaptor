# peer-pod-controller
peer-pod-controller is a kubernetes controller which is responsible for
watching the PeerPodConfig CRD object and manages the creation and deletion
lifecycle of all required components to run peer pods.

## Description
This controller can be run standalone or imported into existing operators. It comes with
a CRD called PeerPodConfig that it's watching. By creating an instance of PeerPodConfig the deployment of
cloud-api-adaptor daemonset and the webhook is triggered, extended resources are advertised by
updating the node capacity fields.

### PeerPodConfig CRD
The PeerPodConfig let's the user specify the number of peer pod vms that can be deployed.
It is spread as evenly as possible across the number of nodes.

## Integrate with your operator
Running the peer-pod-controller as another controller embedded into an operator can be easily
done. Import the controller into your operators main.go and start it.

For an operator-sdk generated operator it could be as simple as:

```go
    import (
        ...
        peerpodcontrollers "github.com/confidential-containers/cloud-api-adaptor/peer-pod-controller/controllers"
        peerpodconfig "github.com/confidential-containers/cloud-api-adaptor/peer-pod-controller/api/v1alpha1"
        ...
    )

    func init() {
        ...
        utilruntime.Must(peerpodconfig.AddToScheme(scheme))
        ...
    }

    func main() {
	...
        if err = (&peerpodcontrollers.PeerPodConfigReconciler{
        	Client: mgr.GetClient(),
        	Log:    ctrl.Log.WithName("controllers").WithName("RemotePodConfig"),
        	Scheme: mgr.GetScheme(),
        }).SetupWithManager(mgr); err != nil {
        	setupLog.Error(err, "unable to create RemotePodConfig controller for OpenShift cluster", "controller", "RemotePodConfig")
        	os.Exit(1)
        }
        ...
    }
```

## Getting Started
Please note that while it is possible to run the controller standalone it is not the
intended use case. 

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/peer-pod-controller:tag
```
	
3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/peer-pod-controller:tag
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
which provides a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster 

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

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
