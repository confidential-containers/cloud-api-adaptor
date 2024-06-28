## Installation

* **Setup Cloud Resources**

  If using AWS, create VPC and AMI. Similarly for other providers create the
  necessary resources.

* **Setup Kubernetes cluster in the cloud**

  At least one node in the cluster must have the "worker" role.
  Verify by executing the following command.
  ```
   kubectl get nodes
  ```
  You should see "worker" under the "ROLES" column as shown below:
  ```
   NAME             STATUS   ROLES                         AGE   VERSION
   testk-master-0   Ready    control-plane,master,worker   37h   v1.25.0
  ```

  If "worker" role is missing, execute the following command to set the role.

    ```
    export NODENAME=<node-name>
    kubectl label node $NODENAME node.kubernetes.io/worker=
    ```

## Deploy webhook

   Please refer to the instructions available in the following [doc](../../webhook/docs/INSTALL.md).

## Configure and deploy CoCo and the cloud-api-adaptor

- Update the `kustomization.yaml` file in `install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml` with your own settings
- Optionally [set up authenticated registry support](../docs/registries-authentication.md)

## Deploy the CoCo operator, CC runtime CRD and the cloud-api-adaptor daemonset

### Using make deploy

You can deploy the CoCo operator and cloud-api-adaptor with the `Makefile` by running
* set CLOUD_PROVIDER
    ```
    export CLOUD_PROVIDER=<aws|azure|ibmcloud|ibmcloud-powervs|libvirt|vsphere>
    ```
    * `RESOURCE_CTRL` is set to `true` by default to allow the peerpod-ctrl to run, monitor and delete dangling cloud resources

* `make deploy` deploys operator, runtime and cloud-api-adaptor pod in the configured cluster
    * validate kubectl is available in your `$PATH` and `$KUBECONFIG` is set
    * `yq` tool is available in your `$PATH`

> **Note:** `make delete` deletes the cloud-api-adaptor daemonset from the configured cluster (and peerpod-ctrl if RESOURCE_CTRL=true is set)

### Manually

Alternatively the manual approach, if you want to pick a specific CoCo release/reference is:

- Deploy the CoCo operator

  <!-- TODO - uncomment when 0.9 is released
  - Either deploy a release version of the peer pods enabled CoCo operator, by running the following command where
  `<RELEASE_VERSION>` needs to be substituted with the desired [release tag](https://github.com/confidential-containers/operator/tags):
  > **Note:** the release version needs to be `v0.9.0` or after
  ```
  export RELEASE_VERSION=<RELEASE_VERSION>
  kubectl apply -k github.com/confidential-containers/operator/config/overlays/peerpods/default?ref=<RELEASE_VERSION>
  ```
  - Alternatively i-->
  - Install the latest development version with:
  ```
  kubectl apply -k "github.com/confidential-containers/operator/config/default"
  ```

- Create the peer pods variant of the CC custom resource to install the required pieces of CC and create the `kata-remote` `RuntimeClass`
    <!-- TODO - uncomment when 0.9 is released
  - Either deploy a release version of the Confidential Containers peer pod customer resource with, by running the following command where `<RELEASE_VERSION>` needs to be substituted with the desired [release tag](https://github.com/confidential-containers/operator/tags):
  > **Note:** the release version needs to be `v0.9.0` or after
  ```
  export RELEASE_VERSION=<RELEASE_VERSION>
  kubectl apply -k github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods?ref=<RELEASE_VERSION>
  ```
  - Alternatively i-->
  - Install the latest development version with:
  ```
  kubectl apply -k "github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods"
  ```
- Wait until all the pods are running with:
  ```
  kubectl get pods -n confidential-containers-system --watch
  ```

- Wait until the `kata-remote` runtime class has been created by running:
  ```
  kubectl get runtimeclass --watch
  ```

- Apply the kustomize.yaml configuration that you modified earlier with:
  ```
  kubectl apply -k install/overlays/ibmcloud
  ```
- Wait until all the pods are running with:
  ```
  kubectl get pods -n confidential-containers-system --watch
  ```
### Verify

* Check POD status

    ```
    kubectl get pods -n confidential-containers-system
    ```
  A successful install should show all the PODs with "Running" status under the `confidential-containers-system`
  namespace.

    ```
    NAME                                              READY   STATUS    RESTARTS   AGE
    cc-operator-controller-manager-546574cf87-phbdv   2/2     Running   0          43m
    cc-operator-daemon-install-pzc4b                  1/1     Running   0          42m
    cc-operator-pre-install-daemon-sgld6              1/1     Running   0          42m
    cloud-api-adaptor-daemonset-mk8ln                 1/1     Running   0          37s
    ```

* View cloud-api-adaptor logs

    ```
    kubectl logs pod/cloud-api-adaptor-daemonset-mk8ln -n confidential-containers-system
    ```

## Building custom cloud-api-adaptor image

* Set CLOUD_PROVIDER
    ```
    export CLOUD_PROVIDER=<aws|azure|ibmcloud|ibmcloud-powervs|libvirt|vsphere>
    ```

* Set container registry and image name
    ```
    export registry=<namespace>/<image_name>
    ```

* Build the container image and push it to `$registry`
   ```
   make image
   ```
