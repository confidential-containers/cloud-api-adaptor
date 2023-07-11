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

## Provision file specifics

As mentioned on the previous section, a properties file can be passed to the cloud provisioner that will be used to controll the provisioning operations. The properties are specific of each cloud provider though, see on the sections below.

### Libvirt provision properties

Use the properties on the table below for Libvirt:

|Property|Description|Default|
|---|---|---|
|libvirt_network|Libvirt Network|"default"|
|libvirt_storage|Libvirt storage pool|"default"|
|libvirt_url|Libvirt connection URI|"qemu+ssh://root@192.168.122.1/system?no_verify=1"|
|libvirt_vol_name|Volume name|"podvm-base.qcow2"|
|libvirt_ssh_key_file|Path to SSH private key||
|pause_image|k8s pause image||
|vxlan_port| VXLAN port number||
|cluster_name|Cluster Name| "peer-pods"|

# Adding support for a new cloud provider

In order to add a test pipeline for a new cloud provider, you will need to implement some
Go interfaces and create a test suite. You will find the reference implementation on the files
for the *libvirt* provider.

## Create the provision implementation

Create a new Go file (.go) named `provision_<CLOUD_PROVIDER>`.go (e.g., `provision_libvirt.go`)
that should be tagged with `//go:build <CLOUD_PROVIDER>`. That file should have the implementation
of the `CloudProvisioner` interface (see its definition in [provision.go](../provisioner/provision.go)).

Apart from that, it should be added an entry to the `GetCloudProvisioner()` factory function in [provision.go](../provisioner/provision.go).

## Create the test suite

Create another Go file named `<CLOUD_PROVIDER>_test.go` to host the test suite and provider-specific assertions. It is interpreted as any [Go standard testing](https://pkg.go.dev/testing) framework test file, where functions with `func TestXxx(*testing.T)` pattern are tests to be executed.

Likewise the provision file, you should tag the test file with `//go:build <CLOUD_PROVIDER>`.

You can have tests specific for the cloud provider or re-use the existing suite found in
[common_suite_test.go](./common_suite_test.go) (or mix both). In the later cases, you must first implement the `CloudAssert` interface (see its definition in [common.go](./common.go)) because some tests will need to do assertions on the cloud side, so there should provider-specific asserts implementations.   

Once you got the assertions done, create the test function which wrap the common suite function. For example, suppose there is a re-usable `doTestCreateSimplePod` test then you can wrap it in test function like shown below:  

```go
func TestCloudProviderCreateSimplePod(t *testing.T) {
	assert := MyAssert{}
	doTestCreateSimplePod(t, assert)
}
```
