# PeerPod setup using Red Hat OpenShift on IBM Cloud

This guide describes how to set up a simple peer pod demo environment with a Red Hat OpenShift Kubernetes cluster on IBM Cloud. This involves the following steps:

1. [Set up an OpenShift Kubernetes cluster for PeerPod VMs](#set-up-an-openshift-kubernetes-cluster-for-peerpod-vms)
1. [Upload a PeerPod VM Custom Image](#upload-a-peerpod-vm-custom-image)
1. [Deploy the PeerPod Webhook](#deploy-the-peerpod-webhook)
1. [Deploy the Confidential-containers operator](#deploy-the-confidential-containers-operator)
1. [Run a Helloworld sample](#run-a-helloworld-sample)

## Pre-reqs

Before proceeding you will need to install:

1. the [Pre-reqs in README.md](./README.md#pre-reqs) but not including Terraform and Ansible, which are not used in this guide.
1. ibmcloud plugins:
    - container-service[kubernetes-service/ks]
    - vpc-infrastructure[infrastructure-service/is]
1. the OpenShift [oc CLI](https://cloud.ibm.com/docs/openshift?topic=openshift-cli-install#install-kubectl-cli)

## Set up an OpenShift Kubernetes cluster for PeerPod VMs

To set up a cluster for peer pod VMs, you need an IBM Cloud VPC, a subnet with a public gateway, and a COS instance.
Set the following environment variables to the values for your setup (note: make sure that your subnet has an attached public gateway):

```bash
export IBMCLOUD_API_KEY=
export VPC_ID=
export SUBNET_ID=
export COS_CRN=
```

Log in to ibmcloud in the region corresponding to your subnet and then set zone and region environnment variables using the following commands:

```bash
export ZONE="$(ibmcloud is subnet $SUBNET_ID -json | jq -r .zone.name)"
export REGION="$(ibmcloud is zone $ZONE -json | jq -r .region.name)"
```

If you have an existing OpenShift cluster, set the following variable to the name of your cluster. Otherwise set it to whatever name you'd like.

```bash
export CLUSTER_NAME=kata-test-roks
```

### Create an Openshift cluster

If you are using an existing cluster, you can skip this section and proceed to [configuring the cluster](#configure-an-openshift-cluster).

1. Create a ROKS cluster

    ```bash
    ibmcloud ks cluster create vpc-gen2 --flavor bx2.4x16 --name "$CLUSTER_NAME" --subnet-id "$SUBNET_ID" --vpc-id "$VPC_ID" --zone "$ZONE" --operating-system RHCOS --workers 2 --version 4.17.12_openshift --disable-outbound-traffic-protection --cos-instance "$COS_CRN"

    ```

    Note: you will need to update the value of the `--version` argument to the current default version returned by `ibmcloud ks versions`, if it is different.

1. Wait until the cluster is completely up and running before proceeding ...

1. Get the `kubeconfig` for your cluster

    ```bash
    ibmcloud ks cluster config --cluster "$CLUSTER_NAME" --admin
    ```

### Configure an OpenShift cluster

By default, your Red Hat OpenShift cluster will not work with the peer pod components. Using the environment variables set in the previous section, proceed with the following steps. 

1. Add a cluster inbound security group rule for the `kata-agent` client

    ```bash
    export CLUSTER_ID=$(ibmcloud ks cluster get --cluster "$CLUSTER_NAME" --output json | jq -r .id)
    export CLUSTER_SG="kube-$CLUSTER_ID"
    export KATA_SG=$(ibmcloud is vpc "$VPC_ID" -json | jq -r .default_security_group.id)
    ibmcloud is sg-rulec "$CLUSTER_SG" inbound udp --port-min 4789 --port-max 4789 --remote "$KATA_SG"
    ```

1. Allow `cloud-api-adaptor` to update pod finalizers

    ```bash
    oc apply -n default -f - <<EOF
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: openshift-caa-finalizer-role
    rules:
    - apiGroups:
      - ""
      resources:
      - "pods/finalizers"
      verbs:
      - "update"
    ---
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: openshift-caa-finalizer-role-binding
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: openshift-caa-finalizer-role
    subjects:
    - kind: ServiceAccount
      name: cloud-api-adaptor
      namespace: confidential-containers-system
    EOF
    ```

1. Label worker nodes for `cloud-api-adaptor`

    ```bash
    oc label nodes $(oc get nodes -o jsonpath={.items..metadata.name}) node.kubernetes.io/worker=
    ```

1. Give `cc-operator` and `cloud-api-adaptor` priviledged OpenShift SCC permission

    ```bash
    oc create namespace confidential-containers-system
    oc project confidential-containers-system
    oc adm policy add-scc-to-user privileged -z cc-operator-controller-manager
    oc adm policy add-scc-to-user privileged -z cloud-api-adaptor
    oc project default
    ```

## Upload a PeerPod VM Custom Image

A peer pod VM image needs to be created as a VPC custom image in IBM Cloud in order to create the peer pod instances
from. You can do this by following the [image instructions in README.md](./README.md#peer-pod-vm-image), or run the following command to use a prebuilt demo image.

> [!WARNING]
> If you have a previously-downloaded image but have since refreshed the cloud-api-adaptor repo, you should re-import the image to make sure you are using an image that is compatible with the latest code.

Run the following command from the root directory of the `cloud-api-adaptor` repository:

```bash
src/cloud-api-adaptor/ibmcloud/image/import.sh ghcr.io/confidential-containers/podvm-generic-ubuntu-amd64:latest "$REGION" --platform linux/amd64
```

This script will end with the line: `Image <image-name> with id <image-id> is available`. Make note of the `image-id`, which will be
needed below.

> [!NOTE]
> If the import.sh script fails and the CLI has not been configured with the COS instance before, you will need to include the `--instance` argument. Refer to [IMPORT_PODVM_TO_VPC.md](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/src/cloud-api-adaptor/ibmcloud/IMPORT_PODVM_TO_VPC.md#running) for details.

## Deploy the PeerPod Webhook

Follow the [webhook instructions in README.md](./README.md#deploy-peerpod-webhook) to deploy cert-manager and the peer-pods webhook.

## Deploy the Confidential-containers operator

The `caa-provisioner-cli` command can be use to simplify deployment of the operator and the cloud-api-adaptor resources on to any cluster. See the [test/tools/README.md](../test/tools/README.md) for full instructions. To create an ibmcloud-ready version of the provisioner CLI, run the following make command:

```bash
# Starting from directory src/cloud-api-adaptor of the cloud-api-adaptor repository
pushd test/tools
make BUILTIN_CLOUD_PROVIDERS="ibmcloud" all
popd
```

This will create `caa-provisioner-cli` in the `src/cloud-api-adaptor/test/tools` directory. To use the command you will need to set up a `.properties` file containing the relevant ibmcloud information to enable your cluster to create and use peer-pods. 

Set the SSH_KEY_ID and PODVM_IMAGE_ID environment variables to your values (Note that the IBMCLOUD_API_KEY, VPC_ID, and SUBNET_ID environment variables should already have been set in [Set up an OpenShift Kubernetes cluster for PeerPod VMs
](#set-up-an-openshift-kubernetes-cluster-for-peerpod-vms)):

```bash
export SSH_KEY_ID= # your ssh key id
export PODVM_IMAGE_ID= # the image id of the peerpod vm uploaded to ibmcloud
#export IBMCLOUD_API_KEY= # your ibmcloud apikey
#export VPC_ID=<your vpc id> # vpc that the cluster is in
#export SUBNET_ID=<your subnet id> # subnet to use (must have a public gateway attached)
```

Then run the following command to generate the `.properties` file:

```bash
cat <<EOF > ~/peerpods-cluster.properties
APIKEY="$IBMCLOUD_API_KEY"
SSH_KEY_ID="$SSH_KEY_ID"
PODVM_IMAGE_ID="$PODVM_IMAGE_ID"
VPC_ID="$VPC_ID"
VPC_SUBNET_ID="$SUBNET_ID"
VPC_SECURITY_GROUP_ID="$(ibmcloud is vpc "$VPC_ID" -json | jq -r .default_security_group.id)"
RESOURCE_GROUP_ID="$(ibmcloud is vpc "$VPC_ID" -json | jq -r .resource_group.id)"
ZONE="$(ibmcloud is subnet $SUBNET_ID -json | jq -r .zone.name)"
REGION="$(ibmcloud is zone $ZONE -json | jq -r .region.name)"
IBMCLOUD_PROVIDER="ibmcloud"
INSTANCE_PROFILE_NAME="bx2-2x8"
CAA_IMAGE_TAG="latest-amd64"
DISABLECVM="true"
EOF
```

This will create a `peerpods-cluster.properties` files in your home directory.

Finally, run the `caa-provisioner-cli` command to install the operator and cloud-api-adaptor:

```bash
export CLOUD_PROVIDER=ibmcloud
export TEST_PROVISION_FILE="$HOME/peerpods-cluster.properties"
export TEST_TEARDOWN="no"
pushd test/tools
./caa-provisioner-cli -action=install
popd
```

Run the following command to confirm that the operator and cloud-api-adaptor have been deployed:

```bash
oc get pods -n confidential-containers-system
```

Once everything is up and ruuning, you should see output similar to the following:

```
cc-operator-controller-manager-7f8db55b55-r9thx   2/2     Running   0          7m55s
cc-operator-daemon-install-2dsnz                  1/1     Running   0          6m59s
cc-operator-daemon-install-d522m                  1/1     Running   0          6m58s
cc-operator-pre-install-daemon-gzl8w              1/1     Running   0          7m29s
cc-operator-pre-install-daemon-w5whl              1/1     Running   0          7m29s
cloud-api-adaptor-daemonset-m5x5s                 1/1     Running   0          7m29s
cloud-api-adaptor-daemonset-v6jdr                 1/1     Running   0          7m29s
peerpod-ctrl-controller-manager-65f76cb59-vhbt4   2/2     Running   0          5m20s
```

## Run a Helloworld sample

You can run the following commands to validate that your cluster has been set up properly and is working as expected.

```bash
oc apply -n default -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/curl/curl.yaml
oc apply -n default -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: helloworld
    version: v1
  name: helloworld
spec:
  containers:
  - name: helloworld
    image: docker.io/istio/examples-helloworld-v1:1.0
    ports:
    - containerPort: 5000
  runtimeClassName: kata-remote
EOF
```

Run the following commands to verify that the pods, peer pod, and pod VM are up and running:

```bash
oc get pods -n default
oc get peerpod -n default
ibmcloud is instances | grep podvm
```

You should see 2 pods, curl and helloworld, but only one peer pod since helloworld is running in a peerpod while
curl is just an oridinary pod that will be used to make a test call to the helloworld service. You should also see
that an IBM Cloud VM instance has been created for the helloworld peer pod.

Finally, run the following command to verify that the helloworld service, running in the peer pod VM, is reachable
from the curl pod:

```bash
export CURL_POD=$(oc get pod -n default -l app=curl -o jsonpath={.items..metadata.name})
export HELLO_IP=$(oc get pod -n default -l app=helloworld -o jsonpath={.items..status.podIP})
oc exec -n default -it $CURL_POD -c curl -- curl http://$HELLO_IP:5000/hello
```

If everything is working, you will see the following output:

```
Hello version: v1, instance: helloworld
```

## Uninstall and clean up

If you want to cleanup the whole demo, including the cluster, simply delete the IBM Cloud cluster. 

> [!NOTE]
> Deleting the cluster might persist the podvm created by cloud-api-adaptor. Make sure to delete the Helloworld pod first.

Otherwise:

1. To delete the Helloworld sample

    ```bash
    oc delete -n default -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/curl/curl.yaml
    oc delete -n default pod helloworld
    ```

1. To uninstall the peer pod components 

    ```bash
    pushd test/tools
    export CLOUD_PROVIDER=ibmcloud
    export TEST_PROVISION_FILE="$HOME/peerpods-cluster.properties"
    ./caa-provisioner-cli -action=uninstall
    oc delete ns confidential-containers-system
    popd
    ```
