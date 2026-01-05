# Installation

## Prerequisites

- validate kubectl is available in your `$PATH` and `$KUBECONFIG` is set to point to your Kubernetes cluster
- `yq` tool is available in your `$PATH`
- At least one node in the cluster must have the "worker" role.

  Verify by executing the following command.

  ```sh
  kubectl get nodes
  ```

  You should see "worker" under the "ROLES" column as shown below:

  ```sh
  NAME             STATUS   ROLES                         AGE   VERSION
  testk-master-0   Ready    control-plane,master,worker   37h   v1.25.0
  ```

  If "worker" role is missing, execute the following command to set the role.

  ```sh
  export NODENAME=<node-name>
  kubectl label node $NODENAME node.kubernetes.io/worker=
  ```

## Deploy CoCo operator and cloud-api-adaptor daemonset

- Update the `kustomization.yaml` file in `install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml` with your own settings
- Optionally [set up authenticated registry support](../docs/registries-authentication.md)
- Install

  ```sh
  export CLOUD_PROVIDER=<aws|azure|gcp|docker|ibmcloud|ibmcloud-powervs|libvirt>
  make deploy
  ```

  This will deploy the latest code from main.

  > **Note:** `make delete` deletes the `cloud-api-adaptor` daemonset and all related pods.

### Installing a specific release version

Take a look at the [tags](https://github.com/confidential-containers/operator/tags) for available releases
and use the specific tag for deployment.

For example if you want to install `v0.11.0` then run the following commands:

  ```sh
  export RELEASE_VERSION=v0.11.0
  kubectl apply -k github.com/confidential-containers/operator/config/default?ref=${RELEASE_VERSION}
  kubectl apply -k github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods?ref=${RELEASE_VERSION}
  ```

> **Note:** the release version needs to be `v0.9.0` or later for the above approach to work.

- Wait until all the pods are running with:

  ```sh
  kubectl get pods -n confidential-containers-system --watch
  ```

- Wait until the `kata-remote` runtime class has been created by running:

  ```sh
  kubectl get runtimeclass --watch
  ```

- Apply the kustomize.yaml configuration that you modified earlier with:

  ```sh
  kubectl apply -k install/overlays/ibmcloud
  ```

- Wait until all the pods are running with:

  ```sh
  kubectl get pods -n confidential-containers-system --watch
  ```

### Verify

- Check POD status

  ```sh
  kubectl get pods -n confidential-containers-system
  ```

  A successful install should show all the PODs with "Running" status under the `confidential-containers-system`
  namespace.

  ```sh
  NAME                                              READY   STATUS    RESTARTS   AGE
  cc-operator-controller-manager-546574cf87-phbdv   2/2     Running   0          43m
  cc-operator-daemon-install-pzc4b                  1/1     Running   0          42m
  cc-operator-pre-install-daemon-sgld6              1/1     Running   0          42m
  cloud-api-adaptor-daemonset-mk8ln                 1/1     Running   0          37s
  ```

- View cloud-api-adaptor logs

  ```sh
  kubectl logs pod/cloud-api-adaptor-daemonset-mk8ln -n confidential-containers-system
  ```

## Building custom cloud-api-adaptor image

- Set CLOUD_PROVIDER

  ```sh
  export CLOUD_PROVIDER=<aws|azure|ibmcloud|ibmcloud-powervs|libvirt>
  ```

- Set container registry and image name

  ```sh
  export registry=<namespace>/<image_name>
  ```

- Build the container image and push it to `$registry`

  ```sh
  make image
  ```
