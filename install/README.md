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
    kubectl label node $NODENAME node-role.kubernetes.io/worker=
    ```

## Deploy webhook

   Please refer to the instructions available in the following [doc](../webhook/docs/INSTALL.md).

## Deploy cloud-api-adaptor

* set CLOUD_PROVIDER
    ```
    export CLOUD_PROVIDER=<aws|azure|ibmcloud|libvirt>
    ```

* `make deploy` deploys operator, runtime and cloud-api-adaptor pod in the configured cluster
    * validate kubectl is available in your `$PATH` and `$KUBECONFIG` is set
    * configure install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml with your own settings
	* [setting up authenticated registry support](../docs/registries-authentication.md)

* `make delete` deletes the daemonset from the configured cluster

### Verify

* Check POD status

    ```
    kubectl get pods -n confidential-containers-system
    ```
  A successful install should show all the PODs with "Running" status under the `confidential-containers-system`
  namespace.
  
    ```
    NAME                                                 READY   STATUS        RESTARTS   AGE
    cc-operator-controller-manager-dc4846d94-nfnr7       2/2     Running       0          20h
    cc-operator-daemon-install-bdp89                     1/1     Running       0          5s
    cc-operator-pre-install-daemon-hclk9                 1/1     Running       0          9s
    cloud-api-adaptor-daemonset-aws-7c66d68484-zpnnw    1/1     Running       0          9s
    ```

* Check `RuntimeClasses`

    ```
    kubectl get runtimeclass
    ```
  A successful install should show `kata` related `RuntimeClasses`
    ```
    NAME        HANDLER     AGE
    kata        kata        6m7s
    kata-clh    kata-clh    6m7s
    kata-qemu   kata-qemu   6m7s
    ```

* View cloud-api-adaptor logs

    ```
    kubectl logs pod/cloud-api-adaptor-daemonset-aws-7c66d68484-zpnnw -n confidential-containers-system
    ```

## Building custom cloud-api-adaptor image

* Set CLOUD_PROVIDER
    ```
    export CLOUD_PROVIDER=<aws|azure|ibmcloud|libvirt>
    ```

* Set container registry and image name
    ```
    export registry=<namespace>/<image_name>
    ```

* Build the container image and push it to `$registry`
   ```
   make image
   ```

## Building custom runtime and pre-install images

   These instructions should help you build your own images for development and testing.

   Before proceeding ensure you can build the kata runtime and the agent successfully by
   following the instructions mentioned in the following [link](../docs/DEVELOPMENT.md).

### Building Runtime Payload Image

* Set container registry and image name
    ```
    export registry=<namespace>/<image_name>
    ```

* Build the container image and push it to `$registry`
    ```
    cd runtime-payload
    make binaries
    make build
    ```


### Building Pre-Install Payload Image

* Set container registry and image name
    ```
    export registry=<namespace>/<image_name>
    ```

* Build the container image and push it to `$registry`
    ```
    cd pre-install-payload
    make build
    ```

