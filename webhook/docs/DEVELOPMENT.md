# Introduction
These instructions should help you to build a custom version of the webhook with your
changes

## Prerequisites
- Golang (1.18.x)
- Operator SDK version (1.23.x+)
- podman and podman-docker or docker
- Access to Kubernetes cluster (1.24+)
- Container registry to store images


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
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.9.1/cert-manager.yaml
```

### Running on the cluster
1. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/<user>/peer-pods-webhook:<tag>
```

2. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/<user>/peer-pods-webhook:<tag>
```
3. To delete the webhook from the cluster:

```sh
make undeploy IMG=<some-registry>/<user>/peer-pods-webhook:<tag>
```

### Testing
You can manually test your changes by following the steps:

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

However, before opening a pull request with your changes, it is recommended that you run
the end-to-end tests locally. Ensure that you have [bats](https://bats-core.readthedocs.io),
docker (or podman-docker), and [kind](https://kind.sigs.k8s.io/) installed in your system;
and then run:
```sh
make test-e2e
```
