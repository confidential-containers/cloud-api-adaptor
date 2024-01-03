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

You should use the `TEST_PROVISION_FILE` variable to specify the properties file path, as shown below:

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

## Provision file specifics

As mentioned on the previous section, a properties file can be passed to the cloud provisioner that will be used to controll the provisioning operations. The properties are specific of each cloud provider though, see on the sections below.

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
|pause_image|Kubernetes pause image||
|podvm_aws_ami_id|AWS AMI ID of the podvm||
|ssh_kp_name|AWS SSH key-pair name ||
|vxlan_port|VXLAN port number||

>Notes:
 * The AWS credentials are obtained from the CLI [configuration files](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html). **Important**: the access key and secret are recorded in plain-text in [install/overlays/aws/kustomization.yaml](https://github.com/confidential-containers/cloud-api-adaptor/tree/main/install/overlays/aws/kustomization.yaml)
 * The subnet is created with CIDR IPv4 block 10.0.0.0/25. In case of deploying an EKS cluster,
a secondary (private) subnet is created with CIDR IPv4 block 10.0.0.128/25
 * The cluster type **onprem** assumes Kubernetes is already provisioned and its kubeconfig file path can be found at the `KUBECONFIG` environment variable or in the `~/.kube/config` file. Whereas **eks** type instructs to create an [AWS EKS](https://aws.amazon.com/eks/) cluster on the VPC
 * You must have `qemu-img` installed in your workstation or CI runner because it is used to convert an qcow2 disk to raw.

### Libvirt provision properties

Use the properties on the table below for Libvirt:

|Property|Description|Default|
|---|---|---|
|libvirt_network|Libvirt Network|"default"|
|libvirt_storage|Libvirt storage pool|"default"|
|libvirt_vol_name|Volume name|"podvm-base.qcow2"|
|libvirt_uri|Libvirt pod URI|"qemu+ssh://root@192.168.122.1/system?no_verify=1"|
|libvirt_conn_uri|Libvirt host URI|"qemu:///system"|
|libvirt_ssh_key_file|Path to SSH private key||
|pause_image|k8s pause image||
|vxlan_port| VXLAN port number||
|cluster_name|Cluster Name| "peer-pods"|

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
## Running tests for PodVM with Authenticated Registry

For running e2e test cases specifically for checking PodVM with Image from Authenticated Registry, we need to export following two variables
- `AUTHENTICATED_REGISTRY_IMAGE` - Name of the image along with the tag from authenticated registry (example: quay.io/kata-containers/confidential-containers-auth:test)
- `REGISTRY_CREDENTIAL_ENCODED` - Credentials of registry encrypted as BASE64ENCODED(USERNAME:PASSWORD). If you're using quay registry, we can get the encrypted credentials from Account Settings >> Generate Encrypted Password >> Docker Configuration

## Running the e2e Test Suite on an Existing CAA Deployment

To test local changes the test suite can run without provisioning any infrastructure, CoCo or CAA. Make sure your cluster is configured and available via kubectl. You also might need to set up Cloud Provider-specific API access, since some of tests assert conditions for cloud resources.

## Azure

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

Run the test suite with the respective flags:

```bash
make test-e2e \
CLOUD_PROVIDER=azure \
TEST_TEARDOWN=no \
TEST_PROVISION=no \
TEST_INSTALL_CAA=no \
TEST_PROVISION_FILE="${PWD}/skip-provisioning.properties" \
```
