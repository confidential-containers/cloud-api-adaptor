# Introduction

The `docker` provider simulates a pod VM inside a docker container.

> **Note**: Run the following commands from the root of this repository.

## Prerequisites

- Install `docker` [engine](https://docs.docker.com/engine/install/) on your K8s worker node

  Ensure docker engine supports API version 1.44+. You can verify it by running 
  `docker version`.
  Docker engine version 26+ supports API 1.44.

  Ensure you complete the [post install steps](https://docs.docker.com/engine/install/linux-postinstall/) if using non-root user

- Install [yq](https://github.com/mikefarah/yq/releases/download/v4.44.2/yq_linux_amd64), [kubectl](https://storage.googleapis.com/kubernetes-release/release/v1.29.4/bin/linux/amd64/kubectl), [kind](https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64), [helm](https://helm.sh/docs/intro/install/) manually or using `prereqs.sh` helper script under `src/cloud-api-adaptor/docker`.

- Kubernetes cluster
```
# The default cluster name is peer-pods if CLUSTER_NAME variable not set
export CLUSTER_NAME={your_cluster_name}
```
use below command to create a kind cluster before deploy CAA
```
./kind_cluster.sh create
```

## Build CAA pod-VM image

- Set environment variables

```bash
export CLOUD_PROVIDER=docker
```

- Build the required pod VM binaries

The same binaries built for the mkosi image is used for the podvm docker image

```bash
cd src/cloud-api-adaptor/podvm-mkosi
make container
```

This will build the required binaries inside a container and place
it under `resources/binaries-tree` and also build the pod VM container image

By default the image is named `quay.io/confidential-containers/podvm-docker-image`.

For quick changes you can just build the binaries of podvm components and
update `./resources/binaries-tree/usr/local/bin` with the new components and
run `make image-container` to build a new podvm container image.

You can download a ready-to-use image on your worker node.

```bash
docker pull quay.io/confidential-containers/podvm-docker-image
```

Note that before you can spin up a pod, the podvm image must be available on the K8s worker node
with the docker engine installed.

## Build CAA container image

> **Note**: If you have made changes to the CAA code and you want to deploy those changes then follow [these instructions](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/src/cloud-api-adaptor/install/README.md#building-custom-cloud-api-adaptor-image) to build the container image from the root of this repository.

## Deploy CAA

The following [`docker.yaml`](../install/charts/peerpods/providers/docker.yaml)
is used, and you can edit it to customize the installation.

### Deploy CAA on the Kubernetes cluster

Run the following command to deploy CAA:

```bash
CLOUD_PROVIDER=docker make deploy
```

Generic CAA deployment instructions are also described [here](../install/README.md).

For changing the CAA image to your custom built image (eg. `quay.io/myuser/cloud-api-adaptor`),
you can use the following:

```bash
export CAA_IMAGE=quay.io/myuser/cloud-api-adaptor
kubectl set image ds/cloud-api-adaptor-daemonset -n confidential-containers-system cloud-api-adaptor-con="$CAA_IMAGE"
```

## Running the CAA e2e tests

### Test Prerequisites

To run the tests, use a test system with at least 8GB RAM and 4vCPUs.
Ubuntu 22.04 has been tested. Other Linux distros should work, but it has not
been tested.

Following software prerequisites are needed on the test system:

- helm
- make
- go
- yq
- kubectl
- kind
- docker

A `prereqs.sh` helper script is available under `src/cloud-api-adaptor/docker` to install/uninstall the prerequisites.


> **Note:**  If using the `prereqs.sh` helper script to install the
> prerequisites, then reload the shell to ensure new permissions
are in place to run `docker` and other commands.

### Test Execution

In order to run the tests, edit the file `src/cloud-api-adaptor/test/provisioner/docker/provision_docker.properties`
and update the `CAA_IMAGE` and `CAA_IMAGE_TAG` variables with your custom CAA image and tag.

You can run the CAA e2e [tests/e2e/README.md](../test/e2e/README.md) by running the following command:

```sh
make TEST_PODVM_IMAGE=<podvm-image> TEST_PROVISION=yes CLOUD_PROVIDER=docker TEST_PROVISION_FILE=$(pwd)/test/provisioner/docker/provision_docker.properties test-e2e
```

This will create a two node kind cluster, automatically download the pod VM
image mentioned in the `provision_docker.properties` file and run the tests. On
completion of the test, the kind cluster will be automatically deleted.

If you want to run the tests on a crio based kind cluster, then update `CONTAINER_RUNTIME` to `crio`
in the `provision_docker.properties` file.

> **Note:**  To overcome docker rate limiting issue or to download images from private registries,
create a `config.json` file under `/tmp` with your registry secrets.

For example:
If your docker registry user is `someuser` and password is `somepassword` then create the auth string
as shown below:

```sh
echo -n "someuser:somepassword" | base64
c29tZXVzZXI6c29tZXBhc3N3b3Jk
```

This auth string needs to be used in `/tmp/config.json` as shown below:

```sh
{
        "auths": {
                "https://index.docker.io/v1/": {
                        "auth": "c29tZXVzZXI6c29tZXBhc3N3b3Jk"
                }
        }
}
```

If you want to use a different location for the registry secret, then remember to update the same
in the `src/cloud-api-adaptor/docker/kind-config.yaml` file if using `containerd` or
in the `src/cloud-api-adaptor/docker/kind-config-crio.yaml` file if using `crio`.

> **Note:** If you have executed the tests with `TEST_TEARDOWN=no`, then you'll
> need to manually delete the `kind` created cluster by running the following
> command:

```sh
kind delete cluster --name peer-pods
```


## Run sample application

### Ensure runtimeclass is present

Verify that the `runtimeclass` is created after deploying CAA:

```bash
kubectl get runtimeclass
```

Once you find a `runtimeclass` named `kata-remote` then you can be sure that the deployment was successful. A successful deployment will look like this:

```console
$ kubectl get runtimeclass
NAME          HANDLER       AGE
kata-remote   kata-remote   7m18s
```

### Deploy workload

Create an `nginx` deployment:

```yaml
echo '
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
      annotations:
        io.containerd.cri.runtime-handler: kata-remote
    spec:
      runtimeClassName: kata-remote
      containers:
      - image: nginx@sha256:9700d098d545f9d2ee0660dfb155fe64f4447720a0a763a93f2cf08997227279
        name: nginx
' | kubectl apply -f -
```

Ensure that the pod is up and running:

```bash
kubectl get pods -n default
```

You could run `docker ps` command to view the docker container running. 
The docker container name will be the pod name prefixed with `podvm`.

Example:

```
$ kubectl get pods
NAME                   READY   STATUS    RESTARTS        AGE
nginx-dbc79c87-jt49h   1/1     Running   1 (3m22s ago)   3m29s

$ docker ps
CONTAINER ID   IMAGE                                                COMMAND                  CREATED         STATUS         PORTS       NAMES
e60b768b847d   quay.io/confidential-containers/podvm-docker-image   "/usr/local/bin/entr…"   3 minutes ago   Up 3 minutes   15150/tcp   podvm-nginx-dbc79c87-jt49h-b9361aef
```

For debugging you can use docker commands like `docker ps`, `docker logs`, `docker exec`.

### Delete workload

```sh
kubectl delete deployment nginx
```

## Troubleshooting

When using `containerd` and `nydus-snapshotter` you might encounter pod creation failure due to
issues with unpacking of image. Check the `nydus-snapshotter` troubleshooting [doc](../docs/troubleshooting/nydus-snapshotter.md).

In order to login to the worker node you can use either of the following approaches

```sh
kubectl debug node/peer-pods-worker -it --image=busybox

# chroot /host
```

or

```sh
docker exec -it peer-pods-worker bash
```
