# Installation

## Prerequisites

- validate kubectl is available in your `$PATH` and `$KUBECONFIG` is set to point to your Kubernetes cluster
- `yq` tool is available in your `$PATH`
- `helm` tool is installed
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

## Deploy cloud-api-adaptor using Helm charts

This project currently uses Helm charts to deploy the kata-deploy chart and
the cloud-api-adaptor components.

This section provides just a quick-start for developers. For detailed installation
instructions, prerequisites, and configuration options, please refer to
the [PeerPods Helm Chart README](./charts/peerpods/README.md).

For development, the easiest way to install it to a given `PROVIDER` is:

- Copy `charts/peerpods/providers/PROVIDER-secrets.yaml.template` to
  `charts/peerpods/providers/PROVIDER-secrets.yaml` and edit the secrets
  properly, unless you are installing for docker.

- Fill `charts/peerpods/providers/PROVIDER.yaml` with required values and any customizations

- Then run the `make deploy` command:
  ```sh
  export CLOUD_PROVIDER=<aws|azure|gcp|docker|ibmcloud|ibmcloud-powervs|libvirt>
  make deploy
  ```

This will deploy the latest code from main. Otherwise if you are in a release tag
then it will deploy the released version, because the containers images will be
pinned to the release version.

> **Note:** `make delete` deletes the `cloud-api-adaptor` daemonset and all related pods.

### Verify

- Check POD status

  ```sh
  kubectl get pods -n confidential-containers-system
  ```

  A successful install should show all the PODs with "Running" status under the `confidential-containers-system`
  namespace.

  ```sh
  NAME                                              READY   STATUS     RESTARTS    AGE
  cloud-api-adaptor-daemonset-wklbv                 1/1     Running    0           15m
  kata-deploy-b5pz2                                 1/1     Running    0           15m
  peerpodctrl-controller-manager-74b5bb8c8b-f2zmm   2/2     Running    0           15m
  ```

  Also the webhook controllers PODs are all "Runnning" under the `peer-pods-webhook-system` namespace.

  ```sh
  NAME                                                    READY   STATUS    RESTARTS   AGE
  peer-pods-webhook-controller-manager-565b98769c-sm78h   2/2     Running   0          18m
  peer-pods-webhook-controller-manager-565b98769c-vrv52   2/2     Running   0          18m
  ```

- View cloud-api-adaptor logs

  ```sh
  kubectl logs -l app=cloud-api-adaptor -n confidential-containers-system
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
