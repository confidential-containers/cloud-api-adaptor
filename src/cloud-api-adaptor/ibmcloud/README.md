# Setup procedure for IBM Cloud

This guide describes how to set up a demo environment on IBM Cloud for peer pod VMs using the operator deployment approach.

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

You will also require [go](https://go.dev/doc/install) and `make` to be installed.

## Peer Pod VM Image

A peer pod VM image needs to be created as a VPC custom image in IBM Cloud in order to create the peer pod instances
from. The peer pod VM image contains components like the agent protocol forwarder and Kata agent that communicate with
the Kubernetes worker node and carry out the received instructions inside the peer pod.

### Building a Peer Pod VM Image via Docker [Optional]

You may skip this step and use one of the release images, skip to [Import Release VM Image](#import-release-vm-image) but for the latest features you may wish to build your own.

You can do this by following the process [document](../podvm/README.md). If building within a container ensure that `--build-arg CLOUD_PROVIDER=ibmcloud` is set and `--build-arg ARCH=s390x` for an `s390x` architecture image.

> **Note:** At the time of writing issue, [#649](https://github.com/confidential-containers/cloud-api-adaptor/issues/649) means when creating an `s390x` image you also need to add two extra
build args: `--build-arg UBUNTU_IMAGE_URL=""` and `--build-arg UBUNTU_IMAGE_CHECKSUM=""`

> **Note:** If building the peer pod qcow2 image within a VM, it may take a lot of resources e.g. 8 vCPU and
32GB RAM due to the nested virtualization performance limitations. When running without enough resources, the failure
seen is similar to:
> ```
> Build 'qemu.ubuntu' errored after 5 minutes 57 seconds: Timeout waiting for SSH.
> ```

#### Upload the built peer pod VM image to IBM Cloud

You can follow the process [documented](./IMPORT_PODVM_TO_VPC.md) from the `cloud-api-adaptor/ibmcloud/image` to extract and upload
the peer pod image you've just built to IBM Cloud as a custom image, noting to replace the
`quay.io/confidential-containers/podvm-ibmcloud-ubuntu-s390x` reference with the local container image that you built
above e.g. `localhost/podvm_ibmcloud_s390x:latest`.

This script will end with the line: `Image <image-name> with id <image-id> is available`. The `image-id` field will be
needed in the kustomize step later.

## Import Release VM Image

Alternatively to use a pre-built peer pod VM image you can follow the process [documented](./IMPORT_PODVM_TO_VPC.md) with the release images found at `quay.io/confidential-containers/podvm-generic-ubuntu-<ARCH>`. Running this command will require docker or podman, as per [tools](./IMPORT_PODVM_TO_VPC.md#tools)

```bash
 ./import.sh quay.io/confidential-containers/podvm-generic-ubuntu-s390x eu-gb --bucket example-bucket --instance example-cos-instance
```

This script will end with the line: `Image <image-name> with id <image-id> is available`. The `image-id` field will be
needed in later steps.


## Create a 'self-managed' Kubernetes cluster on IBM Cloud provided infrastructure
If you don't have a Kubernetes cluster for testing, you can follow the open-source 
[instructions](./cluster)
 to set up a basic cluster where the Kubernetes nodes run on IBM Cloud provided infrastructure.

## Deploy PeerPod Webhook

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
- From within the root directory of the `cloud-api-adaptor` repository, deploy the [webhook](../../webhook/) with:
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

## Deploy the Confidential-containers operator
The `caa-provisioner-cli` simplifies deploying the operator and the cloud-api-adaptor resources on to any cluster. See the [test/tools/README.md](../test/tools/README.md) for full instructions. To create an ibmcloud ready version follow these steps

```bash
# Starting from the cloud-api-adaptor root directory
pushd test/tools
make BUILTIN_CLOUD_PROVIDERS="ibmcloud" all
popd
```

This will create `caa-provisioner-cli` in the `test/tools` directory. To use the tool with an existing self-managed cluster you will need to setup a `.properties` file containing the relevant ibmcloud information to enable your cluster to create and use peer-pods. Use the following commands to generate the `.properties` file, if not using a selfmanaged cluster please update the `terraform` commands with the appropriate values manually.

```bash
export IBMCLOUD_API_KEY= # your ibmcloud apikey
export PODVM_IMAGE_ID= # the image id of the peerpod vm uploaded in the previous step
export PODVM_INSTANCE_PROFILE= # instance profile name that runs the peerpod (bx2-2x8 or bz2-2x8 for example)
export CAA_IMAGE_TAG= # cloud-api-adaptor image tag that supports this arch, see quay.io/confidential-containers/cloud-api-adaptor
pushd ibmcloud/cluster

cat <<EOF > ../../selfmanaged_cluster.properties
IBMCLOUD_PROVIDER="ibmcloud"
APIKEY="$IBMCLOUD_API_KEY"
PODVM_IMAGE_ID="$PODVM_IMAGE_ID"
INSTANCE_PROFILE_NAME="$PODVM_INSTANCE_PROFILE"
CAA_IMAGE_TAG="$CAA_IMAGE_TAG"
SSH_KEY_ID="$(terraform output --raw ssh_key_id)"
REGION="$(terraform output --raw region)"
RESOURCE_GROUP_ID="$(terraform output --raw resource_group_id)"
ZONE="$(terraform output --raw zone)"
VPC_SUBNET_ID="$(terraform output --raw subnet_id)"
VPC_SECURITY_GROUP_ID="$(terraform output --raw security_group_id)"
VPC_ID="$(terraform output --raw vpc_id)"
EOF

popd
```

This will create a `selfmanaged_cluster.properties` files in the cloud-api-adaptor root directory.

The final step is to run the `caa-provisioner-cli` to install the operator.

```bash
export CLOUD_PROVIDER=ibmcloud
# must be run from the directory containing the properties file
export TEST_PROVISION_FILE="$(pwd)/selfmanaged_cluster.properties"
# prevent the test from removing the cloud-api-adaptor resources from the cluster
export TEST_TEARDOWN="no"
pushd test/tools
./caa-provisioner-cli -action=install
popd
```

## End-2-End Test Framework

To validate that a cluster has been setup properly, there is a suite of tests that validate peer-pods across different providers,
the implementation of these tests can be found in [test/e2e/common_suite.go)](../test/e2e/common_suite.go).

Assuming `CLOUD_PROVIDER` and `TEST_PROVISION_FILE` are still set in your current terminal you can execute these tests
from the cloud-api-adaptor root directory by running the following commands

```bash
export KUBECONFIG=$(pwd)/ibmcloud/cluster/config
make test-e2e
```


## Uninstall and clean up

There are two options for cleaning up the environment once testing has finished, or if you want to re-install from a
clean state:
- If using a self-managed cluster you can delete the whole cluster following the
[Delete the cluster documentation](./cluster#delete-the-cluster) and then start again.
- If you instead just want to leave the cluster, but uninstall the Confidential Containers and peer pods
feature, you can use the `caa-provisioner-cli` to remove the resources.

```bash
export CLOUD_PROVIDER=ibmcloud
# must be run from the directory containing the properties file
export TEST_PROVISION_FILE="$(pwd)/selfmanaged_cluster.properties"
pushd test/tools
./caa-provisioner-cli -action=uninstall
popd
```
