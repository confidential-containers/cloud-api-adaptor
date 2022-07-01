# Introduction
This mutating webhook modifies a POD spec using specific runtimeclass to remove all `resources` entries and replace it with peer-pod extended resource.

## Need for mutating webhook
A peer-pod uses resources at two places:
- Kubernetes Worker Node: Peer-Pod metadata, Kata shim resources, remote-hypervisor/cloud-api-adaptor resources, vxlan etc
- Cloud Instance: The actual peer-pod VM running in the cloud (eg. EC2 instance in AWS, or Azure VM instance)

For peer-pods case the resources are really consumed outside of the worker node. It’s external to the Kubernetes cluster. 

This creates two problems:
1. Peer-pod scheduling can fail due to the unavailability of required resources on the worker node even though the peer-pod will not consume the requested resources from the worker node.

2. Cluster-admin have no way to view the actual peer-pods VM capacity and consumption.


A simple solution to the above problems is to advertise peer-pod capacity as Kubernetes extended resources and let Kubernetes scheduler handle the peer-pod capacity tracking and accounting. Additionally, POD overhead can be used to account for actual `cpu` and `mem` resource requirements on the Kubernetes worker node. 
The mutating webhook removes any `resources` entries from the Pod spec and adds the peer-pods extended resources.


![](https://i.imgur.com/MYwSQaX.png)



## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.

You'll also need to advertise the extended resource `kata.peerpods.io/vm`.

A simple daemonset is provided under the following [directory](./hack/extended-resources/).

```
cd ./hack/extended-resources
./setup.sh
```

### Using kind cluster
For `kind` clusters, you can use the following Makefile targets

Create kind cluster
```
make kind-cluster
```
Deploy the webhook in the kind cluster
```
make kind-deploy IMG=<some-registry>/<user>/peer-pods-webhook:<tag>
```

If not using `kind`, the follow these steps to deploy the webhook

### Deploy cert-manager
```
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.6.1/cert-manager.yaml
```

### Running on the cluster
1. Build and push your image to the location specified by `IMG`:

```sh
make docker-build && make docker-push IMG=<some-registry>/<user>/peer-pods-webhook:<tag>
```

2. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/<user>/peer-pods-webhook:<tag>
```
3. To delete the webhook from the cluster:

```sh
make undeploy
```

### Testing
1. Create the runtimeclass
```sh
kubectl apply -f hack/rc.yaml
```
2. Create the pod
```sh
kubectl apply -f hack/pod.yaml
```
3. View the mutated pod
```sh
kubectl get -f hack/pod.yaml -o yaml | grep kata.peerpods
```

You can see that that the `hack/pod.yaml` has Kubernetes resources specified in the spec:
```
resources:
  requests:
    cpu: 1
    memory: 1Gi
  limits:
    cpu: 1
    memory: 2Gi
```
In the mutated pod these have been removed and the pod overhead
```
  overhead:
    cpu: 250m
    memory: 120Mi
```

and peer pod vm limit added.
```
 resources:
   limits:
     kata.peerpods.io/vm: "1"
   requests:
     kata.peerpods.io/vm: "1"
```

