## Installation

*  **Setup Cloud Resources**

  If using AWS, create VPC and AMI. Similarly for other providers create the
  necessary resources.
   
* **Setup Kubernetes cluster in the cloud**

  If using a single node cluster then label the node with "worker" role.
   
    ```
    kubectl label node $NODENAME node-role.kubernetes.io/worker=
    ```

* **Deploy the `Confidential Containers` operator**

    ```
    kubectl apply -f https://raw.githubusercontent.com/confidential-containers/cloud-api-adaptor/staging/install/yamls/deploy.yaml
    ```

* **Create a `ConfigMap` with the cloud provider settings**

  Example configmap providing parameters for the AWS provider.
  Note the `name`, `namespace` and usage of `hyp.env` as the file name in the  data section. These values must not be changed.

    ```
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: hyp-env-cm
      namespace: confidential-containers-system
    data:
      hyp.env: |
          CAA_PROVIDER="aws"
          AWS_ACCESS_KEY_ID="ABC"
          AWS_SECRET_ACCESS_KEY="12345"
          AWS_REGION="us-east-2"
    ```

  Example configmap providing parameters for the Libvirt provider.

    ```
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: hyp-env-cm
      namespace: confidential-containers-system
    data:
      hyp.env: |
          CAA_PROVIDER="libvirt"
          LIBVIRT_URI="qemu+ssh://root@192.168.122.1/system"
          LIBVIRT_NET="kubernetes"
          LIBVIRT_POOL="kubernetes"
    ```

* **Create CCruntime Custom Resource (CR)** 

    ```
    kubectl apply -f https://raw.githubusercontent.com/confidential-containers/cloud-api-adaptor/staging/install/yamls/ccruntime-peer-pods.yaml
    ```

## Verify

* Check POD status

    ```
    kubectl get pods -n confidential-containers-system
    ```
  A successful install should show all PODs with "Running" status
  
    ```
    NAME                                             READY   STATUS        RESTARTS   AGE
    cc-operator-controller-manager-dc4846d94-nfnr7   2/2     Running       0          20h
    cc-operator-daemon-install-bdp89                 1/1     Running       0          5s
    cc-operator-pre-install-daemon-hclk9             1/1     Running       0          9s
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

* Login to the worker node and verify the status remote hypervisor service

    ```
    sudo systemctl status remote-hyp.service
    ```

    It should be in running state.


## Building runtime and pre-install images

   These instructions should help you build your own images for development and testing


### Building Runtime Payload Image

    Set container registry and image name
    ```
    export REGISTRY=<NAMESPACE>/<IMAGE_NAME>
    ```

    Build the container image
    ```
    cd runtime-payload
    make binaries
    make build
    ```


### Building Pre-Install Payload Image

    Set container registry and image name
    ```
    export REGISTRY=<NAMESPACE>/<IMAGE_NAME>
    ```

    Build the container image
    ```
    cd pre-install-payload
    make build
    ```

