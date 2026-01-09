# Introduction

This directory contain the framework to run complete end-to-end (e2e) tests for single Provider. It was built upon the [kubernetes-sigs e2e framework](https://github.com/kubernetes-sigs/e2e-framework).

# Running end-to-end tests

>**Note:** When running the e2e framework, both the `kubectl` and `git` commands needs to be available on `PATH`.

As long as the cloud provider support is implemented on this framework, you can run the tests
as shown below for *libvirt*:

```
$ CLOUD_PROVIDER=libvirt make test-e2e
```

The above command run tests on an existing cluster. It will look for the kubeconf file exported on the
`KUBECONFIG` variable, and then in `$HOME/.kube/config` if not found.

You can instruct the tool to provision a test environment though, as shown below:

```
$ TEST_PROVISION=yes CLOUD_PROVIDER=libvirt make test-e2e
```

Each provider must have a provisioner implementation so that the framework is able to perform operations on the cluster. The provisioner will likely to need additional information (e.g. login credentials), and those are passed via a properties file with the following format:

```
key1 = "value1"
key2 = "value2"
...
```

You should use the `TEST_PROVISION_FILE` variable to specify the properties file path, as shown below (for libvirt provider look at [libvirt/README.md](../../libvirt/README.md) setup):

```
$ TEST_PROVISION=yes TEST_PROVISION_FILE=/path/to/libvirt.properties CLOUD_PROVIDER=libvirt make test-e2e
```

The `TEST_PODVM_IMAGE` is an optional variable which specifies the path to the podvm qcow2 image. If it is set then the image should be uploaded to the VPC storage. The following command, as an example, instructs the tool to upload `path/to/podvm-base.qcow2` after the provisioning of the test environment:

```
$ TEST_PROVISION=yes TEST_PODVM_IMAGE="path/to/podvm-base.qcow2" CLOUD_PROVIDER=libvirt make test-e2e
```

By default it is given 20 minutes for the entire e2e execution to complete, otherwise the process is preempted. If you need to extend that timeout then export the `TEST_E2E_TIMEOUT` variable. For example, `TEST_E2E_TIMEOUT=30m` set the timeout to 30 minutes. See `-timeout` in [go test flags](https://pkg.go.dev/cmd/go#hdr-Testing_flags) for the values accepted.

To leave the cluster untouched by the execution finish you should export `TEST_TEARDOWN=no`, otherwise the framework will attempt to wipe out any resources created. For example, if `TEST_PROVISION=yes` is used to create a VPC and cluster for testing and `TEST_TEARDOWN=no` not specified, then at the end of the test the provisioned cluster and VPC, will both be deleted.

To use existing cluster which have already installed Cloud API Adaptor, you should export `TEST_INSTALL_CAA=no`.

While in development and/or debugging it's common that you want to run just a sub-set of tests rather than the entire suite. To accomplish that
you should export an unanchored regular expression in the `RUN_TESTS` variable that matches the tests names (see `-run` in [go test flags](https://pkg.go.dev/cmd/go#hdr-Testing_flags) for the regular expression format accepted). For example, to run only the simple creation pod test:
```
$ RUN_TESTS=CreateSimplePod TEST_PROVISION=yes TEST_PODVM_IMAGE="path/to/podvm-base.qcow2" CLOUD_PROVIDER=libvirt make test-e2e
```

## Attestation and KBS specific

We need artifacts from the trustee repo when doing the attestation tests.
To prepare trustee, execute the following helper script:

```sh
${cloud-api-adaptor-repo-dir}/src/cloud-api-adaptor/test/utils/checkout_kbs.sh
```
> [!NOTE]
> This script requires [oras](https://oras.land/docs/installation/) to be installed to pull down and verify
the cached kbs-client.


We need build and use the PodVM image:

```sh
pushd ${cloud-api-adaptor}
make podvm-builder podvm-binaries podvm-image
popd
```

Then extract the PodVM image and use it following [extracting-the-qcow2-image](../../podvm/README.md#extracting-the-qcow2-image)

To deploy the KBS service and test attestation related cases, export the following variable:

```sh
export DEPLOY_KBS=yes
```

## Other end-to-end test customizations

Other options are provided via environment variables if you need to further customize the e2e test cases:
- `TEST_CAA_NAMESPACE` - This option is available, primarily for running the e2e tests on a downstream version
of confidential containers, where the cloud-api-adaptor pod is deployed to a different namespace than the default
 `confidential-containers-system`.

# Running end-to-end tests against pre-configured cluster

Let's say you want to run the e2e tests against a pre-configured cluster in Azure:

```sh
TEST_PROVISION=no TEST_INSTALL_CAA=no make CLOUD_PROVIDER=azure TEST_PROVISION_FILE=azure_test.properties test-e2e
```

If your environment already has [Trustee operator](https://github.com/confidential-containers/trustee-operator) configured for attestation, then you can run e2e with the following command:

```sh
TEST_TRUSTEE_OPERATOR=yes TEST_PROVISION=no TEST_INSTALL_CAA=no make CLOUD_PROVIDER=azure TEST_PROVISION_FILE=azure_test.properties test-e2e
```

If your environment uses a pod VM image with restrict agent policy, then some of the e2e tests may fail.
You can use the following command to override the test pods with a relaxed agent policy allowing all APIs.

```sh
POD_ALLOW_ALL_POLICY_OVERRIDE=yes TEST_PROVISION=no TEST_INSTALL_CAA=no make CLOUD_PROVIDER=azure TEST_PROVISION_FILE=azure_test.properties test-e2e
```

The `POD_ALLOW_ALL_POLICY_OVERRIDE` variable will not override the policy for a test pod if the `io.katacontainers.config.agent.policy` already exists in the pod spec.

## Provision file specifics

As mentioned on the previous section, a properties file can be passed to the cloud provisioner that will be used to control the provisioning operations. The properties are specific of each cloud provider though, see on the sections below.

### AWS provision properties

Use the properties on the table below for AWS:

|Property|Description|Default|
|---|---|---|
|aws_region|AWS region|Account default|
|aws_vpc_cidrblock|AWS VPC CIDR block|10.0.0.0/24|
|aws_vpc_id|AWS VPC ID||
|aws_vpc_igw_id|AWS VPC Internet Gateway ID||
|aws_vpc_rt_id|AWS VPC Route Table ID||
|aws_vpc_sg_id|AWS VPC Security Groups ID||
|aws_vpc_subnet_id|AWS VPC Subnet ID||
|cluster_type|Kubernetes cluster type. Either **onprem** or **eks** (see Notes below) |onprem|
|container_runtime|Test cluster configured container runtime. Either **containerd** or **crio** |containerd|
|disablecvm|Set to `true` to disable confidential VM||
|pause_image|Kubernetes pause image||
|peerpods_secret_name|Name of the Kubernetes secret for AWS credentials. When set, Helm will use reference mode (`secrets.mode=reference`) instead of direct injection. If empty, credentials are passed directly via Helm values||
|podvm_aws_ami_id|AWS AMI ID of the podvm||
|ssh_kp_name|AWS SSH key-pair name ||
|use_public_ip|Set `true` to instantiate VMs with public IP. If `cluster_type=onprem` then this property is implictly applied||
|tunnel_type|Tunnel type||
|vxlan_port|VXLAN port number||

>Notes:
 * The AWS credentials are obtained from the CLI [configuration files](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html). **Important**: the access key and secret are recorded in plain-text in [install/overlays/aws/kustomization.yaml](../../install/overlays/aws/kustomization.yaml)
 * The subnet is created with CIDR IPv4 block 10.0.0.0/25. In case of deploying an EKS cluster,
a secondary (private) subnet is created with CIDR IPv4 block 10.0.0.128/25
 * The cluster type **onprem** assumes Kubernetes is already provisioned and its kubeconfig file path can be found at the `KUBECONFIG` environment variable or in the `~/.kube/config` file. Whereas **eks** type instructs to create an [AWS EKS](https://aws.amazon.com/eks/) cluster on the VPC
 * You must have `qemu-img` installed in your workstation or CI runner because it is used to convert an qcow2 disk to raw.

### Libvirt provision properties

Use the properties on the table below for Libvirt:

|Property|Description|Default|
|---|---|---|
|container_runtime|Test cluster configured container runtime. Either **containerd** or **crio** |containerd|
|libvirt_network|Libvirt Network|"default"|
|libvirt_storage|Libvirt storage pool|"default"|
|libvirt_vol_name|Volume name|"podvm-base.qcow2"|
|libvirt_uri|Libvirt pod URI|"qemu+ssh://root@192.168.122.1/system?no_verify=1"|
|libvirt_conn_uri|Libvirt host URI|"qemu:///system"|
|libvirt_ssh_key_file|Path to SSH private key||
|pause_image|k8s pause image||
|tunnel_type|Tunnel type||
|vxlan_port| VXLAN port number||
|cluster_name|Cluster Name| "peer-pods"|

## Running tests for PodVM with Authenticated Registry

For running e2e test cases specifically for checking PodVM with Image from Authenticated Registry, we need to export following two variables
- `AUTHENTICATED_REGISTRY_IMAGE` - Name of the image along with the tag from authenticated registry (example: quay.io/kata-containers/confidential-containers-auth:test)
- `REGISTRY_CREDENTIAL_ENCODED` - Credentials of registry encrypted as BASE64ENCODED(USERNAME:PASSWORD). If you're using quay registry, we can get the encrypted credentials from Account Settings >> Generate Encrypted Password >> Docker Configuration

## Running the e2e Test Suite on an Existing CAA Deployment

To test local changes the test suite can run without provisioning any infrastructure, CoCo or CAA. Make sure your cluster is configured and available via kubectl. You also might need to set up Cloud Provider-specific API access, since some of tests assert conditions for cloud resources.

### Azure

Fill in `RESOURCE_GROUP` and `AZURE_SUBSCRIPTION_ID` with the values you want to use in your test:

```bash
cd ../.. # go to project root
cat <<EOF> skip-provisioning.properties
RESOURCE_GROUP_NAME="..."
AZURE_SUBSCRIPTION_ID="..."
AZURE_CLIENT_ID="unused"
AZURE_TENANT_ID="unused"
LOCATION="unused"
AZURE_IMAGE_ID="unused"
EOF
```

Run the test suite with the respective flags. Enable `TEST_KBS` and provide a `KBS_ENDPOINT` to run KBS-related test.

```bash
make test-e2e \
CLOUD_PROVIDER=azure \
TEST_TEARDOWN=no \
TEST_PROVISION=no \
TEST_INSTALL_CAA=no \
TEST_PROVISION_FILE="${PWD}/skip-provisioning.properties" \
TEST_KBS=yes \
KBS_ENDPOINT=http://10.224.0.5:30362 \
RUN_TESTS=TestAzureImageDecryption
```

### IBM Cloud
Take region `jp-tok` for example.
```
cd ../.. # go to project root
cat <<EOF> skip-provisioning.properties
REGION="jp-tok"
ZONE="jp-tok-1"
VPC_ID="<vpc-of-worker>"
VPC_SUBNET_ID="<subnet-of-worker>"
VPC_SECURITY_GROUP_ID="<security-group-of-vpc>"
RESOURCE_GROUP_ID="<resource-group-id>"
IBMCLOUD_PROVIDER="ibmcloud"
APIKEY="<your-ibmcloud-apikey>"

IAM_SERVICE_URL="https://iam.cloud.ibm.com/identity/token"
VPC_SERVICE_URL="https://jp-tok.iaas.cloud.ibm.com/v1"
IKS_SERVICE_URL="https://containers.cloud.ibm.com/global"
PODVM_IMAGE_ID="<podvm-image-uploaded-previously>"
INSTANCE_PROFILE_NAME="bz2-2x8"
PODVM_IMAGE_ARCH="s390x"
IMAGE_PULL_API_KEY="<can-be-same-as-apikey>"
CAA_IMAGE_TAG="<caa-image-tag>"
EOF
```

- For `INSTANCE_PROFILE_NAME`, if it's not secure execution, the value is started with "bz2". If it's secure execution, the value is started with 'bz2e'. More values can be found through ibmcloud command `ibmcloud is instance-profiles`.
- For `PODVM_IMAGE_ID`, the vpc image id uploaded to ibmcloud.
- For `CAA_IMAGE_TAG`, the commit id of project. The commit id can be found here: https://github.com/confidential-containers/cloud-api-adaptor/commits/main/

# Adding support for a new cloud provider

In order to add a test pipeline for a new cloud provider, you will need to implement some
Go interfaces and create a test suite. You will find the reference implementation on the files
for the *libvirt* provider.

## Create the provision implementation

Create a folder named <CLOUD_PROVIDER> and create a new Go file (.go) named `provision`.go under it (e.g., `libvirt/provision.go`)
that should be tagged with `//go:build <CLOUD_PROVIDER>`. That file should have the implementation
of the `CloudProvisioner` interface (see its definition in [provision.go](../provisioner/provision.go)).

Apart from that, it should be added an entry to the `GetCloudProvisioner()` factory function in [provision.go](../provisioner/provision.go).

## Create the test suite

Create another Go file named `<CLOUD_PROVIDER>_test.go` to host the test suite and provider-specific assertions. It is interpreted as any [Go standard testing](https://pkg.go.dev/testing) framework test file, where functions with `func TestXxx(*testing.T)` pattern are tests to be executed.

Likewise the provision file, you should tag the test file with `//go:build <CLOUD_PROVIDER>`.

You can have tests specific for the cloud provider or re-use the existing suite found in
[common_suite.go](./common_suite.go) (or mix both). In the later cases, you must first implement the `CloudAssert` interface (see its definition in [common.go](./common.go)) because some tests will need to do assertions on the cloud side, so there should provider-specific asserts implementations.

Once you got the assertions done, create the test function which wrap the common suite function. For example, suppose there is a re-usable `DoTestCreateSimplePod` test then you can wrap it in test function like shown below:

```go
func TestCloudProviderCreateSimplePod(t *testing.T) {
    assert := MyAssert{}
    DoTestCreateSimplePod(t, assert)
}
```
