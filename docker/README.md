# Introduction

The `docker` provider simulates a pod VM inside a docker container.

> **Note**: Run the following commands from the root of this repository.

## Prerequisites

- Install `docker` [engine](https://docs.docker.com/engine/install/)
- Kubernetes cluster

## Build CAA pod-VM image

- Set environment variables

```bash
export CLOUD_PROVIDER=docker
```

- Build the required pod VM binaries
 
```bash
cd docker
make
```

This will build the required binaries inside a container and place 
it under `resources/binaries-tree`

- Build the pod VM image

```bash
make image
cd ..
```

This will build the podvm docker image. By default the image is named `quay.io/confidential-containers/podvm-docker-image`.

You can download a ready-to-use image on your host.

```bash
docker pull quay.io/confidential-containers/podvm-docker-image
```

Note that before you can spin up a pod, the podvm image needs to be available on the docker host

## Build CAA container image

> **Note**: If you have made changes to the CAA code and you want to deploy those changes then follow [these instructions](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/install/README.md#building-custom-cloud-api-adaptor-image) to build the container image from the root of this repository.

If you would like to deploy the latest code from the default branch (`main`) of this repository then expose the following environment variable:

```bash
export registry="quay.io/confidential-containers"
```

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