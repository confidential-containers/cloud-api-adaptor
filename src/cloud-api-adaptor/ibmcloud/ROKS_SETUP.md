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
    ibmcloud ks cluster create vpc-gen2 --flavor bx2.4x16 --name "$CLUSTER_NAME" --subnet-id "$SUBNET_ID" --vpc-id "$VPC_ID" --zone "$ZONE" --operating-system RHCOS --workers 2 --version 4.17.14_openshift --disable-outbound-traffic-protection --cos-instance "$COS_CRN"

    ```

    Note: you will need to update the value of the `--version` argument to the current default version returned by `ibmcloud ks versions`, if it is different.

1. Wait until the cluster is completely up and running before proceeding ...

1. Get the `kubeconfig` for your cluster

    ```bash
    ibmcloud ks cluster config --cluster "$CLUSTER_NAME" --admin
    ```

### Configure an OpenShift cluster

By default, your Red Hat OpenShift cluster will not work with the peer pod components. Using the environment variables set in the previous section, proceed with the following steps. 

1. Add security group rules to allow traffic between the cluster and peer pod VSIs

    ```bash
    export CLUSTER_ID=$(ibmcloud ks cluster get --cluster "$CLUSTER_NAME" --output json | jq -r .id)
    export CLUSTER_SG="kube-$CLUSTER_ID"
    export VPC_SG=$(ibmcloud is vpc "$VPC_ID" -json | jq -r .default_security_group.id)
    # Add a cluster inbound security group rule for the kata-agent client
    ibmcloud is sg-rulec "$CLUSTER_SG" inbound udp --port-min 4789 --port-max 4789 --remote "$VPC_SG"
    # Add a VPC inbound security group rule for the cluster client
    ibmcloud is sg-rulec "$VPC_SG" inbound all --remote "$CLUSTER_SG"
    ```

1. Allow your peer pod VSIs to send traffic through the VPE gateway to access services like `icr.io`

    ```bash
    export VPEGW_SG="kube-vpegw-$VPC_ID"
    export CIDR=$(ibmcloud is subnet $SUBNET_ID --output json | jq -r '.ipv4_cidr_block')
    ibmcloud is sg-rulec $VPEGW_SG inbound tcp --remote $CIDR
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

A peer pod VM image needs to be available as a VPC custom image in IBM Cloud to create the peer pod instances with. If you want to run the full confidential containers end-to-end demo, including TDX attestation with a [Trustee](https://github.com/confidential-containers/trustee), you will need to make sure that your peer pod VM image is configured with the TDX attestation agent and kernel modules.

If you don't have a suitable peer pod image, you will need to [build one](./README.md#peer-pod-vm-image). For example, you can use the following command to build a TDX enabled RHEL image:
```bash
# Run this command in directory src/cloud-api-adaptor
PODVM_DISTRO=rhel TEE_PLATFORM=tdx ACTIVATION_KEY=<key> ORG_ID=<org id> IMAGE_URL=<path to base kvm qcow2 image> make podvm-builder podvm-binaries podvm-image
```

This will create the RHEL image wrapped in a docker container image. You can then upload the RHEL image to IBM Cloud by running the following command from the root directory of the `cloud-api-adaptor` repository:
```bash
src/cloud-api-adaptor/ibmcloud/image/import.sh <built docker image>:<image tag> "$REGION" --pull never --os red-9-amd64
```

> [!TIP]
> If you don't have a TDX enabled image and are unable to build one, you can still run the peer pod demo without attestation. Run the following command to import a prebuilt non-TDX demo image:
> ```bash
> src/cloud-api-adaptor/ibmcloud/image/import.sh ghcr.io/confidential-containers/podvm-generic-ubuntu-amd64:latest "$REGION" --platform linux/amd64
> ```

The import script will end with the line: `Image <image-name> with id <image-id> is available`. Make note of the `image-id`, which will be
needed below.

> [!NOTE]
> If the import.sh script fails and the CLI has not been configured with the COS instance before, you will need to include the `--instance` argument. Refer to [IMPORT_PODVM_TO_VPC.md](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/src/cloud-api-adaptor/ibmcloud/IMPORT_PODVM_TO_VPC.md#running) for details.

## Deploy the PeerPod Webhook

Follow the [webhook instructions in README.md](./README.md#deploy-peerpod-webhook) to deploy cert-manager and the peer-pods webhook.

## Deploy the Confidential-containers operator

The `caa-provisioner-cli` command can be use to simplify deployment of the operator and the cloud-api-adaptor resources on to any cluster. See the [test/tools/README.md](../test/tools/README.md) for full instructions. To create an ibmcloud-ready version of the provisioner CLI, run the following make command:

```bash
# Starting from root directory of the cloud-api-adaptor repository
pushd src/cloud-api-adaptor/test/tools
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

> [!TIP]
> You can configure IAM for the cloud api adaptor using a [Trusted Profile](https://cloud.ibm.com/docs/account?topic=account-create-trusted-profile&interface=ui), instead of an API key. To do so, replace the line `APIKEY="$IBMCLOUD_API_KEY"` with `IAM_PROFILE_ID="the_id_of_your_trusted_profile"`, in the following `.properties` file.

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

You can optionally run peer pods in confidential (TDX enabled) VMs by changing the `DISABLECVM` property to `false`, but make sure you also change the `INSTANCE_PROFILE_NAME` property to a profile that supports the TDX confidential computing mode. For example:

```bash
sed -i ".bak" -e 's/DISABLECVM="true"/DISABLECVM="false"/' -e 's/bx2-2x8/bx3dc-2x10/' ~/peerpods-cluster.properties
```

> [!WARNING]
> If you want to experiment with attestation, you need to set `INITDATA="<your initdata>"` to reference a [Trustee](https://github.com/confidential-containers/trustee) service that is configured to verify TDX evidence. You also need to make sure that your configured peer pod VM image includes the TDX attestation agent and kernel modules.

> [!TIP]
> You can configure and run a simple Trustee in another VSI in your VPC by following the instructions in [Deploy a test Trustee](#deploy-a-test-trustee) before proceeding.

Finally, run the `caa-provisioner-cli` command to install the operator and cloud-api-adaptor:

```bash
export CLOUD_PROVIDER=ibmcloud
export TEST_PROVISION_FILE="$HOME/peerpods-cluster.properties"
export TEST_TEARDOWN="no"
pushd src/cloud-api-adaptor/test/tools
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
export HELLO_IP=$(oc get pod -n default helloworld -o jsonpath={.status.podIP})
oc exec -n default -it $CURL_POD -c curl -- curl http://$HELLO_IP:5000/hello
```

If everything is working, you will see the following output:

```
Hello version: v1, instance: helloworld
```

> [!NOTE]
> If you have a Trustee configured, you can also test that attestation is working by running curl inside the helloworld pod to retrieve a key from the confidential data hub (CDH).
>
> For example, if your Trustee is configured with the same example key as used in the [test Trustee](#deploy-a-test-trustee), you can retrieve the value of key1 using the following command:
> ```bash
> oc exec -n default -it helloworld -- bash
> curl http://127.0.0.1:8006/cdh/resource/default/kbsres1/key1
> ```
> If it is working, this will output the key's configured value:
> ```
> res1val1
> ```

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
    pushd src/cloud-api-adaptor/test/tools
    export CLOUD_PROVIDER=ibmcloud
    export TEST_PROVISION_FILE="$HOME/peerpods-cluster.properties"
    ./caa-provisioner-cli -action=uninstall
    popd
    ```

## Deploy a test Trustee

The following instructions can be used to set up a simple Trustee with an HTTP endpoint for testing attestation.

1. Create a Ubuntu VSI in the same VPC as your ROKS cluster.
    ```
    ibmcloud is instance-create "$CLUSTER_NAME-trustee" "$VPC_ID" "$ZONE" "bx2-2x8" "$SUBNET_ID" --image "r014-85b1a9ec-369b-41d6-b921-39666d4139d1" --keys "$SSH_KEY_ID" --allow-ip-spoofing false
    ```

1. SSH into the new VSI and run the following commands.
    > Tip: you can SSH to the private IP of the VSI from a pod or node in your ROKS cluster, since they are running in the same VPC.

    First, run the following commands to install docker and oras:
    ```
    sudo apt-get update
    sudo apt-get install ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}") stable" | \
      sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update
    sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin -y
    sudo snap install oras --classic
    ```

    Next, deploy the Trustee services:
    ```
    git clone https://github.com/confidential-containers/trustee.git
    cd trustee
    openssl genpkey -algorithm ed25519 > kbs/config/private.key
    openssl pkey -in kbs/config/private.key -pubout -out kbs/config/public.pub
    sudo docker compose up -d
    ```

    Finally, configure an example key that can be used for CDH testing:
    ```
    oras pull ghcr.io/confidential-containers/staged-images/kbs-client:latest
    chmod +x kbs-client
    cat > kbsres1_key1 << EOF
    res1val1
    EOF
    ./kbs-client --url http://127.0.0.1:8080 config --auth-private-key kbs/config/private.key set-resource --resource-file kbsres1_key1 --path default/kbsres1/key1
    ```

1. Configure your confidential containers environment to use the Trustee.

    First, use the VSI IP to set the `KBS_SERVICE_ENDPOINT` environment variable to the URL of the Trustee:
    ```
    export KBS_SERVICE_ENDPOINT="http://$(ibmcloud is instance "$CLUSTER_NAME-trustee" --output JSON | jq -r '.network_interfaces[].primary_ip.address'):8080"
    ```

    Then, set the `INITDATA` environment variable to the compressed and encoded Trustee configuration:
    ```
    export INITDATA=$(cat <<EOF | gzip | base64 -w0
    algorithm = "sha256"
    version = "0.1.0"

    [data]
    "aa.toml" = '''
    [token_configs]
    [token_configs.coco_as]
    url = "$KBS_SERVICE_ENDPOINT"

    [token_configs.kbs]
    url = "$KBS_SERVICE_ENDPOINT"
    '''

    "cdh.toml"  = '''
    socket = 'unix:///run/confidential-containers/cdh.sock'
    credentials = []

    [kbc]
    name = "cc_kbc"
    url = "$KBS_SERVICE_ENDPOINT"
    '''
    EOF
    )
    ```

    If you want to use this Trustee for all peer pods in the cluster, run the following command to configure the global `INITDATA` property in your `peerpods-cluster.properties` file:
    ```
    echo "INITDATA=\"$INITDATA\"" >> ~/peerpods-cluster.properties
    ```

    Alternatively, you can configure the Trustee for a specific peer pod, by including the `io.katacontainers.config.runtime.cc_init_data` annotation on the pod. For example:
    ```
    apiVersion: v1
    kind: Pod
    metadata:
      name: mypod
      annotations:
        io.katacontainers.config.runtime.cc_init_data: $INITDATA
    spec:
      runtimeClassName: kata-remote
      ...
    ```
