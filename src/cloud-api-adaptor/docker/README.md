# Introduction

The `docker` provider simulates a pod VM inside a docker container.

> **Note**: Run the following commands from the root of this repository.

## Prerequisites

- Install `docker` [engine](https://docs.docker.com/engine/install/) on your K8s worker node

  Ensure docker engine supports API version 1.44+. You can verify it by running 
  `docker version`.
  Docker engine version 26+ supports API 1.44.

  Ensure you complete the [post install steps](https://docs.docker.com/engine/install/linux-postinstall/) if using non-root user
  
  
- Kubernetes cluster

## Build CAA pod-VM image

- Set environment variables

```bash
export CLOUD_PROVIDER=docker
```

- Build the required pod VM binaries
 
```bash
cd src/cloud-api-adaptor/docker/image
make
```

This will build the required binaries inside a container and place 
it under `resources/binaries-tree`

- Build the pod VM image

```bash
make image
cd ../../
```

This will build the podvm docker image. By default the image is named `quay.io/confidential-containers/podvm-docker-image`.

For quick changes you can just build the binaries of podvm components and update `./resources/binaries-tree/usr/local/bin` with the new
components and run `make image` to build a new podvm image.

You can download a ready-to-use image on your worker node.

```bash
docker pull quay.io/confidential-containers/podvm-docker-image
```

Note that before you can spin up a pod, the podvm image must be available on the K8s worker node
with the docker engine installed.


## Build CAA container image

> **Note**: If you have made changes to the CAA code and you want to deploy those changes then follow [these instructions](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/src/cloud-api-adaptor/install/README.md#building-custom-cloud-api-adaptor-image) to build the container image from the root of this repository.

## Deploy CAA

The following [`kustomization.yaml`](../install/overlays/docker/kustomization.yaml) is used.


### Deploy CAA on the Kubernetes cluster

Run the following command to deploy CAA:

```bash
CLOUD_PROVIDER=docker make deploy
```

Generic CAA deployment instructions are also described [here](../install/README.md).

For changing the CAA image to your custom built image (eg. `quay.io/myuser/cloud-api-adaptor`),
you can use the following:

```bash
kubectl set image ds/cloud-api-adaptor-daemonset -n confidential-containers-system cloud-api-adaptor-con=quay.io/myuser/cloud-api-adaptor
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

