# Setup procedure for IBM Cloud

This guide describes how to set up a demo environment on IBM Cloud for peer pod VMs using the operator deployment approach.

> **Note:** The previous approach that used terraform and ansible to install peer pods and build the cloud-api-adaptor
and peer pod vm image as part of a single process is deprecated in favour of the operator (which aligns with the other
 cloud providers), but can currently be found in the [terraform directory](./terraform/README.md).

The high level flow involved is:

- Build and upload a peer pod custom image to IBM Cloud
- Create a 'self-managed' Kubernetes cluster on IBM Cloud provided infrastructure
- Deploy Confidential-containers operator
- Deploy and validate that the nginx demo works
- Clean-up and deprovision

## Pre-reqs

When building the peer pod VM image, it is simplest to use the container based approach, which only requires either
`docker`, or `podman`, but it can also be built locally.

> **Note:** the peer pod VM image build and upload is de-coupled from the cluster creation and operator deployment stage,
so can be built on a different machine.

There are a number of packages that you will need to install in order to create the Kubernetes cluster and peer pod enable it:
- Terraform, Ansible, the IBM Cloud CLI and `kubectl` are all required for the cluster creation and explained in
the [cluster pre-reqs guide](./cluster/README.md#prerequisites).

In addition to this you will need to install [`jq`](https://stedolan.github.io/jq/download/)
> **Tip:** If you are using Ubuntu linux, you can run follow command:
> ```bash
> $ sudo apt-get install jq
> ```

## Build and upload a peer pod VM image to IBM Cloud

A peer pod VM image needs to be created as a VPC custom image in IBM Cloud in order to create the peer pod instances
from. The peer pod VM image contains components like the agent protocol forwarder and Kata agent that communicate with
the Kubernetes worker node and carry out the received instructions inside the peer pod.

#### Build the peer pod VM image

Once the peer pod feature has been released it is likely that some example peer pod VM images will be published for use,
but at the moment you will need to build your own. You can do this by following the process 
[documented](../podvm/README.md). If building within a container ensure that `--build-arg CLOUD_PROVIDER=ibmcloud` is
set and `--build-arg ARCH=s390x` for an `s390x` architecture image.
> **Note:** At the time of writing issue, [#649](https://github.com/confidential-containers/cloud-api-adaptor/issues/649) means when creating an `s390x` image you also need to add two extra
build args: `--build-arg UBUNTU_IMAGE_URL=""` and `--build-arg UBUNTU_IMAGE_CHECKSUM=""`

> **Note:** If building the peer pod qcow2 image within a VM, it may take a lot of resources e.g. 8 vCPU and
32GB RAM due to the nested virtualization performance limitations. When running without enough resources, the failure
seen is similar to:
> ```
> Build 'qemu.ubuntu' errored after 5 minutes 57 seconds: Timeout waiting for SSH.
> ```

#### Upload the peer pod VM image to IBM Cloud

You can follow the process [documented](./IMPORT_PODVM_TO_VPC.md) from the `cloud-api-adaptor/ibmcloud/image` to extract and upload
the peer pod image you've just built to IBM Cloud as a custom image, noting to replace the
`quay.io/confidential-containers/podvm-ibmcloud-ubuntu-s390x` reference with the local container image that you built
above e.g. `localhost/podvm_ibmcloud_s390x:latest`.

This script will end with the line: `Image <image-name> with id <image-id> is available`. The `image-id` field will be
needed in the kustomize step later.

## Create a 'self-managed' Kubernetes cluster on IBM Cloud provided infrastructure
If you don't have a Kubernetes cluster for testing, you can follow the open-source 
[instructions](./cluster)
 to set up a basic cluster where the Kubernetes nodes run on IBM Cloud provided infrastructure.

After creating the cluster and setting up `kubeconfig` as described, ensure that the node has the worker label set. To do
this you can run:
```
node=$(kubectl get nodes -o json | jq -r '.items[-1].metadata.name')
kubectl label node $node node-role.kubernetes.io/worker=
```

## Deploy the Confidential-containers operator

#### Deploy cert-manager
- Deploy cert-manager with:
  ```
  kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.9.1/cert-manager.yaml
  ```
- Wait for the pods to all be in running state with:
  ```
  kubectl get pods -n cert-manager --watch
  ```

#### Deploy the peer-pods webhook
- From within the root directory of the `cloud-api-adaptor` repository, deploy the [webhook](../webhook) with:
  ```
  kubectl apply -f ./webhook/hack/webhook-deploy.yaml
  ``` 
- Wait for the pods to all be in running state with:
  ```
  kubectl get pods -n peer-pods-webhook-system --watch
  ```

- Advertise the extended resource `kata.peerpods.io/vm.` by running the following commands:
  ```
  pushd webhook/hack/extended-resources
  ./setup.sh
  popd
  ```

#### Configure kustomization.yaml

Kustomize is an approach allowing Kubernetes configurations to be updated whilst allowing reuse of the base files
across all environments (and cloud provider for peer pods). It is the place that the IBM Cloud peer pod VM 
configuration is supplied. From the root directory of the `cloud-api-adaptor` repository, run the following steps to
set up environment variables.

- Export the output values from the self-managed cluster set-up:
  ```
  pushd ibmcloud/cluster
  export SSH_KEY_ID=$(terraform output --raw ssh_key_id)
  export VPC_ID=$(terraform output --raw vpc_id)
  export SUBNET_ID=$(terraform output --raw subnet_id)
  export SECURITY_GROUP_ID=$(terraform output --raw security_group_id)
  export REGION=$(terraform output --raw region)
  export ZONE=$(terraform output --raw zone)
  export RESOURCE_GROUP=$(terraform output --raw resource_group_id)
  popd
  ```

- Set up the other required environment variables:
  ```
  export IBMCLOUD_API_KEY="<your api key>"
  export PODVM_IMAGE_ID="<the id of the image uploaded to IBM Cloud earlier>"
  export INSTANCE_PROFILE_NAME="<the instance profile name for the peer pod vm>"
  export CAA_TAG=$(git rev-parse HEAD 2>/dev/null)
  ```

> **Note:** for `INSTANCE_PROFILE_NAME`, the value depends on the architecture of the peer pods image you which to 
create. The values are typically:
>  - `"bx2-2x8"` for a `amd64` architecture peer pod
>  - `"bz2-2x8"` for a `s390x` architecture peer pod
>  - `"bz2e-2x8"` for a Secure Execution peer pod

> **Note:** for `CAA_TAG`, this command will link to the latest commit of the cloud-api-adaptor, which should be available
to pull from the [quay confidential containers image repository](https://quay.io/repository/confidential-containers/cloud-api-adaptor?tab=tags),
but there could be a small timing window when the image hasn't be uploaded yet. In addition if you are working on a fork/branch of then this won't match, so should look at the registry to find the most recent build tag.
It is worth noting that the `latest` and `dev-<commit>` tags of the `cloud-api-adaptor` image do not have `s390x` support.

- Using the environment variables set above, update the ibmcloud kustomize file:
  ```
  target_file_path="install/overlays/ibmcloud/kustomization.yaml"
  sed -i "s%newTag:.*%newTag: \"${CAA_TAG}\"%" ${target_file_path}
  sed -i "s%newName:.*%newName: quay.io/confidential-containers/cloud-api-adaptor%" ${target_file_path}
  sed -i "s%IBMCLOUD_VPC_ENDPOINT=.*%IBMCLOUD_VPC_ENDPOINT="https://""${REGION}.iaas.cloud.ibm.com/v1"%" ${target_file_path}
  sed -i "s%IBMCLOUD_RESOURCE_GROUP_ID=.*%IBMCLOUD_RESOURCE_GROUP_ID="${RESOURCE_GROUP}"%" ${target_file_path}
  sed -i "s%IBMCLOUD_SSH_KEY_ID=.*%IBMCLOUD_SSH_KEY_ID="${SSH_KEY_ID}"%" ${target_file_path}
  sed -i "s%IBMCLOUD_PODVM_IMAGE_ID=.*%IBMCLOUD_PODVM_IMAGE_ID="${PODVM_IMAGE_ID}"%" ${target_file_path}
  sed -i "s%IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME=.*%IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME="${INSTANCE_PROFILE_NAME}"%" ${target_file_path}
  sed -i "s%IBMCLOUD_ZONE=.*%IBMCLOUD_ZONE="${ZONE}"%g" ${target_file_path}
  sed -i "s%IBMCLOUD_VPC_SUBNET_ID=.*%IBMCLOUD_VPC_SUBNET_ID="${SUBNET_ID}"%" ${target_file_path}
  sed -i "s%IBMCLOUD_VPC_SG_ID=.*%IBMCLOUD_VPC_SG_ID="${SECURITY_GROUP_ID}"%" ${target_file_path}
  sed -i "s%IBMCLOUD_VPC_ID=.*%IBMCLOUD_VPC_ID="${VPC_ID}"%" ${target_file_path}
  sed -i "s%IBMCLOUD_API_KEY=.*%IBMCLOUD_API_KEY="${IBMCLOUD_API_KEY}"%" ${target_file_path}
  sed -i "s%IBMCLOUD_IAM_ENDPOINT=.*%IBMCLOUD_IAM_ENDPOINT="https://iam.cloud.ibm.com/identity/token"%" ${target_file_path}
  ```

#### Deploy the operator, runtime and cloud-api-adaptor pod
- Deploy the peer pods version of the CoCo controller manager with:
  ```
  kubectl apply -f install/yamls/deploy.yaml
  ``` 
- Wait for the cc-operator-controller-manager be in running state with:
  ```
  kubectl get pods -n confidential-containers-system --watch
  ```

- Apply the kustomize with:
  ```
  kubectl apply -k install/overlays/ibmcloud
  ```
- Wait until all the pods are running with:
  ```
  kubectl get pods -n confidential-containers-system --watch
  ```

- Wait until the kata runtime class has been created by running:
  ```
  kubectl get runtimeclass --watch
  ```

## Validating the set-up

- Apply a test nginx pod by running:
  ```
  kubectl apply -f ibmcloud/demo/nginx.yaml
  ```
- Wait for it to be running with:
  ```
  kubectl get pods --watch
  ```

- Exec into the pod and check the config map was mounted correctly with:
  ```
  kubectl exec pod/nginx -- cat /etc/config/example.txt
  ```
  and check that is shows `Hello, world!`

- Delete the test pod to clean-up with:
  ```
  kubectl delete -f ibmcloud/demo/nginx.yaml
  ```

## Uninstall and clean up

There are two options for cleaning up the environment once testing has finished, or if you want to re-install from a
clean state:
- If using a self-managed cluster you can delete the whole cluster following the
[Delete the cluster documentation](./cluster#delete-the-cluster) and then start again.
- If you instead just want to leave the cluster, but uninstall the Confidential Containers and peer pods
feature, the following commands can be run:
  - Delete the cloud-api-adaptor pods with:
    ```
    kubectl delete -k install/overlays/ibmcloud
    ```
  - Delete the confidential containers controller manager with:
    ```
    kubectl delete -f install/yamls/deploy.yaml
    ``` 
    - Wait for the all the confidential containers pods to to finish terminating and be removed with:
    ```
    kubectl get pods -n confidential-containers-system --watch
    ```
  - Delete the webhook with:
    ```
    kubectl delete -f ./webhook/hack/webhook-deploy.yaml
    ``` 
    - Check that the webhook pod have been removed with:
    ```
    kubectl get pods -n peer-pods-webhook-system
    ```
  - Delete cert-manager with:
    ```
    kubectl delete -f https://github.com/jetstack/cert-manager/releases/download/v1.9.1/cert-manager.yaml
    ```
    - Check that the cert-manager pods have been removed with:
    ```
    kubectl get pods -n cert-manager
    ```