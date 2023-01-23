# Introduction

This directory contains the sources to build the podvm image (qcow2 file) for various Linux distributions and cloud providers. So use
the instructions in the next sections if you need to build your own image with changes to meet your requirements. Otherwise you can
find [here](../docs/consuming-prebuilt-podvm-images.md) information on how to consume pre-built images.

# How to build locally

In order to build locally it requires the source trees and softwares mentioned in the [developer's guide](../docs/DEVELOPMENT.md) to build this project binaries. It will also need [packer](https://www.packer.io/) (to build the qcow2), [rust](https://www.rust-lang.org/tools/install) (to build the Kata Containers's agent), as well as the following packages:

* On Ubuntu:

  ```$ apt-get install -y qemu-kvm cloud-utils qemu-utils protobuf-compiler pkg-config libdevmapper-dev libgpgme-dev```

You may need to link the agent with the musl C library. In this case, you should install the musl-tools (Ubuntu) package and setup the Rust toolchain as explained [here](https://github.com/kata-containers/kata-containers/blob/CCv0/src/agent/README.md#build-with-musl).

Finally run the following commands to build the qcow2 image:

```
$ export CLOUD_PROVIDER=[aws|azure|ibmcloud|libvirt]
$ make image
```

# How to build within container

This directory contains dockerfiles to build the podvm image entirely within container so that it only requires docker or podman installed on the host.

In general it is needed to follow these steps:

1. Build a builder container image for a given Linux distribution
1. Build the podvm container image for a given cloud provider
1. Extract the podvm image (qcow2 file) from the container image

The next sections describe that process in details. Note that although the following examples use docker, it can be carried out with podman too.

## Building a builder image

The builder image packages the cloud-api-adaptor and Kata Containers sources as well as the softwares to build
the binaries (e.g. *kata-agent* and *agent-protocol-forwarder*) that should be installed in the podvm image.

The builder image is agnostic to cloud providers in the sense that one can be used to build for multiple providers, however it is
dependent on the Linux distribution the image is built for. Therefore, in this directory you will find dockerfiles for each supported distributions, which are currently Ubuntu 20.04 ([Dockerfile.podvm_builder](./Dockerfile.podvm_builder)), CentOS Stream 8 ([Dockerfile.podvm_builder.centos](./Dockerfile.podvm_builder.centos)), and RHEL 8.7 ([Dockerfile.podvm_builder.rhel](./Dockerfile.podvm_builder.rhel)).

As an example, to build the builder image for Ubuntu, run:

```
$ docker build -t podvm_builder -f Dockerfile.podvm_builder .
```

Use `--build-arg` to pass build arguments to docker to overwrite default values if needed. Following are the arguments
currently accepted:

|Argument|Default value|Description|
|--------|-------------|-----------|
|CAA\_SRC |https://github.com/confidential-containers/cloud-api-adaptor | The cloud-api-adaptor source repository |
|CAA\_SRC\_BRANCH|staging| cloud-api-adaptor repository branch |
|KATA\_SRC | https://github.com/kata-containers/kata-containers | The Kata Containers source repository |
|KATA\_SRC\_BRANCH | CCv0 | The Kata Containers repository branch |
|GO\_VERSION | 1.18.7 | Go version |
|PROTOC\_VERSION | 3.11.4 | [Protobuf](https://github.com/protocolbuffers/protobuf) version |
|RUST\_VERSION | 1.66.0 | Rust version |

As it can be noted in the table above the cloud-api-adaptor repository is cloned within the builder image, so rather than
copying the local source tree, it will be using the upstream source. But if you want to test local changes then you should:

* Push the changes to your fork in github (e.g. https://github.com/$USER/cloud-api-adaptor/tree/my-changes-in-a-branch).
* Overwrite the *CAA_SRC* and *CAA_SRC_BRANCH* arguments as shown below:

```
$ docker build -t podvm_builder \
	--build-arg CAA_SRC=https://github.com/$USER/cloud-api-adaptor \
	--build-arg CAA_SRC_BRANCH=my-changes-in-a-branch \
	-f Dockerfile.podvm_builder .
```

## Building the podvm qcow2 image

The process to build the podvm container image will effectively compile binaries and create the qcow2 file.

In order to build the podvm image you should be using the corresponding dockerfile of the Linux distro for which the builder image was built. For example, if the builder image was built with *Dockerfile.podvm_builder.DISTRO* then you should use the *Dockerfile.podvm.DISTRO* to build the podvm image.

The builder image has to be indicated via `BUILDER_IMG` build argument to docker. Apart from that, the `CLOUD_PROVIDER` argument is mandatory and specifies the cloud provider that binaries should be built for. Once again, one builder image can be used to build for multiple
cloud providers and architectures.

Below is shown how to build an Ubuntu image for libvirt:

```
$ docker build -t podvm_libvirt \
	--build-arg BUILDER_IMG=podvm_builder:latest \
	--build-arg CLOUD_PROVIDER=libvirt \
	-f Dockerfile.podvm .
```

This step will take several minutes to complete, mostly because `packer` will use the QEMU builder in emulation mode when running within container. If you are using podman then you can speed up QEMU by enabling native virtualization, i.e., passing the `--device=/dev/kvm` argument to enable KVM inside the container.

Also beware that the process consume a bunch of memory and disk from the host. As a tip, if the build fail at the point QEMU was launched but packer couldn't connect via ssh then it might indicate lack of memory (try to increase the amount of memory if running on VM).

The podvm image can be built for other architectures than x86\_64 by passing the `ARCH` build argument to docker. Currently this is only supported for Ubuntu s390x as shown below:

```
$ docker build -t podvm_libvirt_s390x \
	--build-arg ARCH=s390x \
	--build-arg BUILDER_IMG=podvm_builder:latest \
	--build-arg CLOUD_PROVIDER=libvirt \
	-f Dockerfile.podvm .
```

## Extracting the qcow2 image

The final podvm image, i.e. the qcow2 file, is stored on the root of the podvm container image.

There are a couple of ways to extract files from a container image using docker or podman. However, to ease that task
we have the [hack/download-image.sh](hack/download-image.sh) script, which copy the qcow2 file out of the podvm
container image. Use it as shown below:

```
$ ./hack/download-image.sh podvm_libvirt:latest . -o podvm.qcow2
```

# How to add support for a new Linux distribution

In order to add a new Linux distribution essentially it is needed to create some dockerfiles and add the packer configuration files.

Follow the steps below, replacing `DISTRO` with the name of the distribution being added:

1. Create the builder dockerfile by copying `Dockerfile.podvm_builder` to `Dockerfile.podvm_builder.DISTRO` and
   adjusting the file properly (e.g. replace `FROM ubuntu:20.04` with `FROM DISTRO`). Try to keep the same
   software versions (e.g. Golang, Rust) as much as possible.
1. Create the podvm image dockerfile by copying `Dockerfile.podvm` to `Dockerfile.podvm.DISTRO` and adjusting the file
   properly likewise. In particular, the *PODVM_DISTRO* and *BUILDER_IMG* arguments should be changed.
1. Create the packer directory (`mkdir qcow2/DISTRO`) where the `qemu-DISTRO.pkr.hcl` and `variables.pkr.hcl` files should be placed. Also on this step you can also use an existing configuration (e.g. `qcow2/ubuntu`) as a template. Ensure that common scripts and files like the `qcow2/misc-settings.sh` are adjusted to support the DISTRO if needed.
1. Define the base image URL and checksum in the `Makefile` file.
1. Update this *README.md* properly in case that there are specific instructions and/or constraints for the DISTRO.
