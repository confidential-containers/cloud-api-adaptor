## Installation

*  **Setup Cloud Resources**

  If using AWS, create VPC and AMI. Similarly for other providers create the
  necessary resources.
   
* **Setup Kubernetes cluster in the cloud**

  If using a single node cluster then label the node with "worker" role.
   
    ```
    kubectl label node $NODENAME node-role.kubernetes.io/worker=
    ```

## Build and install with cloud-api-adaptor running in a pod

* set CLOUD_PROVIDER
    ```
    export CLOUD_PROVIDER=<aws|ibmcloud|libvirt>
    ```

* `make image` builds the container image and push it to `$registry`
* `make deploy` deploys operator, runtime and cloud-api-adaptor pod in the configured cluster
    * configure install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml with your own settings
    * validate kubectl is available in your `$PATH` and `$KUBECONFIG` is set
* `make delete` deletes the pod deployment from the configured cluster

### Verify

* Check POD status

    ```
    kubectl get pods -n confidential-containers-system
    ```
  A successful install should show all PODs with "Running" status
  
    ```
    NAME                                                 READY   STATUS        RESTARTS   AGE
    cc-operator-controller-manager-dc4846d94-nfnr7       2/2     Running       0          20h
    cc-operator-daemon-install-bdp89                     1/1     Running       0          5s
    cc-operator-pre-install-daemon-hclk9                 1/1     Running       0          9s
    cloud-api-adaptor-deployment-aws-7c66d68484-zpnnw    1/1     Running       0          9s
    ```

* Check `RuntimeClasses`

    ```
    kubectl get runtimeclass
    ```
  A successful install should show the following `RuntimeClasses`
    ```
    NAME        HANDLER     AGE
    kata        kata        6m7s
    kata-clh    kata-clh    6m7s
    kata-qemu   kata-qemu   6m7s
    ```

* View cloud-api-adaptor logs

    ```
    kubectl logs pod/cloud-api-adaptor-deployment-aws-7c66d68484-zpnnw -n confidential-containers-system
    ```



## Building runtime and pre-install images

   These instructions should help you build your own images for development and testing.

   Before proceeding ensure you can build the kata runtime and the agent successfully by
   following the instructions mentioned in the following [link](../docs/DEVELOPMENT.md)

### Building Runtime Payload Image

* Set container registry and image name
    ```
    export REGISTRY=<NAMESPACE>/<IMAGE_NAME>
    ```

* Build the container image
    ```
    cd runtime-payload
    make binaries
    make build
    ```


### Building Pre-Install Payload Image

* Set container registry and image name
    ```
    export REGISTRY=<NAMESPACE>/<IMAGE_NAME>
    ```

* Build the container image
    ```
    cd pre-install-payload
    make build
    ```

