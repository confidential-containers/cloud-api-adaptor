# Introduction

This directory contains the sources to build the podvm image (qcow2 file) for various Linux distributions and cloud providers. So use
the instructions in the next sections if you need to build your own image with changes to meet your requirements. Otherwise you can
find [here](../docs/consuming-prebuilt-podvm-images.md) information on how to consume pre-built images.

# How to build locally

In order to build locally it requires the source trees and softwares mentioned in the [developer's guide](../docs/DEVELOPMENT.md) to build this project binaries. It will also need [packer](https://www.packer.io/) (to build the qcow2) as well as the following packages:

* On Ubuntu:

  ```bash
  $ apt-get install -y qemu-kvm cloud-utils qemu-utils protobuf-compiler pkg-config libdevmapper-dev libgpgme-dev
  ```

Finally run the following commands to build the qcow2 image:

```bash
$ export CLOUD_PROVIDER=[aws|azure|ibmcloud|libvirt|generic]
$ make image
```
**NOTE:** "generic" is a best-effort provider agnostic image creation

# How to build within container

This directory contains dockerfiles to build the podvm image entirely within container so that it only requires docker or podman installed on the host.

In general it is needed to follow these steps:

1. Build a builder container image for a given Linux distribution
2. Build an image containing all the required podvm binaries for a given Linux distribution
3. Build an image containing the podvm qcow2 image for a given cloud provider
4. Extract the podvm image (qcow2 file) from the container image

The next sections describe that process in details. Note that although the following examples use docker, it can be carried out with podman too.

## Building a builder image

The builder image packages the cloud-api-adaptor and Kata Containers sources as well as the softwares to build
the binaries (e.g. *kata-agent* and *agent-protocol-forwarder*) that should be installed in the podvm image.

The builder image is agnostic to cloud providers in the sense that one can be used to build for multiple providers, however it is
dependent on the Linux distribution the image is built for. Therefore, in this directory you will find dockerfiles for each
supported distributions, which are currently Ubuntu 24.04 ([Dockerfile.podvm_builder](./Dockerfile.podvm_builder)),
Fedora 39 ([Dockerfile.podvm_builder.fedora](./Dockerfile.podvm_builder)) and RHEL 9
([Dockerfile.podvm_builder.rhel](./Dockerfile.podvm_builder.rhel)).

You can create the builder image using the make target by running:
```bash
$ make -C .. podvm-builder
```

You can optionally customize the builder image, by specify shell variables to the `make` command:
| Variable            | Default value  | Description                                                     |
| ------------------- | -------------- | --------------------------------------------------------------- |
| `ARCH`              | `amd64`/`s390x`| Architecture of the podvm image to be built. Defaults to the architecture the of the current machine |
| `PODVM_DISTRO`      | `ubuntu`       | Valid options are `ubuntu`, `fedora` and `rhel` |
| `ORG_ID`            | `""`           | rhel only: the organization ID for Red Hat Subscription Management (RHSM) |
| `ACTIVATION_KEY`    | `""`           | rhel only: the activation key for Red Hat Subscription Management (RHSM)  |

e.g. to produce an s390x architecture builder image
```
ARCH=s390x make -C .. podvm-builder
```

## Building the image containing the podvm binaries

Like the builder image, we have make targets for the binaries image in the parent directory.

> **Note:** The `BUILDER_IMG` environment variable is crucial as it specifies
> the builder image, which is the result of the previous step build. Ensure you
> have built the builder image before proceeding.

To build the binaries image, use the following command:

```bash
$ BUILDER_IMG=<your_builder_image> make -C .. podvm-binaries
```

The build process can take significant time.

You can customize the build, by using some environment variables like ARCH,
PODVM_DISTRO, and other things. For details visit the Makefile.

## Building the podvm qcow2 image

In order to build the podvm image you should be using the corresponding
dockerfile of the Linux distro for which the builder and binaries image were
built.  For example, if the builder image was built with
*Dockerfile.podvm_builder.DISTRO* then you should use the
*Dockerfile.podvm.DISTRO* to build the podvm image.

The builder image has to be indicated via `BUILDER_IMG` build argument and
binaries image has to be indicated via `BINARIES_IMG` build argument to docker.

Below command will build the qcow2 image that can be used for all cloud providers
based on Ubuntu distro.

```bash
$ docker build -t podvm \
         --build-arg BUILDER_IMG=podvm_builder \
         --build-arg BINARIES_IMG=podvm_binaries \
         -f Dockerfile.podvm .
```

This step will take several minutes to complete, mostly because `packer` will
use the QEMU builder in emulation mode when running within container.
> **Tip:** If you are using podman then you can speed up QEMU by enabling native
> virtualization, by passing the `--device=/dev/kvm` argument to enable KVM inside
> the container.

> **Note:** Beware that the process consume a bunch of memory and disk from the host.
If the build fails at the point QEMU was launched but packer couldn't
connect via ssh, with an error similar to:
> ```
> Build 'qemu.ubuntu' errored after 5 minutes 57 seconds: Timeout waiting for SSH.
> ```
> then it might indicate lack of memory, so try to increase the amount of memory if running on VM.

The podvm image can be built for other architectures than `x86_64` by passing
the `ARCH` build argument to docker:

```bash
$ docker build -t podvm_s390x \
         --build-arg ARCH=s390x \
         --build-arg BUILDER_IMG=podvm_builder \
         --build-arg BINARIES_IMG=podvm_binaries_s390x \
         -f Dockerfile.podvm .
```

The Secure Execution enabled podvm image can be built by passing the `SE_BOOT` build argument to docker. Currently this is only supported for Ubutu `s390x`, which also needs put the `HOST KEY documents` to the [files](files) folder, please follow the `Download host key document from Resource Link` section at [this document](../ibmcloud/SECURE_EXECUTION.md) to download `HOST KEY documents`.
```bash
$ tree -L 1 files
files
├── HKD-8562-1234567.crt
├── etc
└── usr
```
Running below command will build the Secure Execution enabled qcow2 image:
```bash
$ docker build -t se_podvm_s390x \
         --build-arg ARCH=s390x \
         --build-arg SE_BOOT=true \
         --build-arg BUILDER_IMG=podvm_builder \
         --build-arg BINARIES_IMG=podvm_binaries_s390x \
         -f Dockerfile.podvm .
```

The podvm image can also be built using UEFI based images. For example if you want to build a
RHEL podvm image using UEFI based qcow2 image, then run the build using as shown below:

```
# RHEL Dockerfile supports in passing an image file, file has to be in the docker context
$ docker build -t podvm-uefi \
         --build-arg BUILDER_IMG=podvm_builder \
         --build-arg BINARIES_IMG=podvm_binaries \
         --build-arg UEFI=true \
         --build-arg IMAGE_CHECKSUM="_qcow2_image_checksum" \
         --build-arg IMAGE_URL="uefi.qcow2" \
         -f Dockerfile.podvm.rhel .
```

## Extracting the qcow2 image

The final podvm image, i.e. the qcow2 file, is stored on the root of the podvm
container image.

There are a couple of ways to extract files from a container image using docker
or podman. However, to ease that task we have the
[hack/download-image.sh](hack/download-image.sh) script, which copy the qcow2
file out of the podvm container image.

Running the below command will extract the qcow2 image built in the previous step.

```bash
$ ./hack/download-image.sh podvm:latest . -o podvm.qcow2
```
Running the below command will extract the Secure Execution enabled qcow2 image built in the previous step.

```bash
$ ./hack/download-image.sh se_podvm_s390x:latest . -o se_podvm.qcow2
```

# How to add support for a new Linux distribution

In order to add a new Linux distribution essentially it is needed to create some dockerfiles and add the packer configuration files.

Follow the steps below, replacing `DISTRO` with the name of the distribution being added:

1. Create the builder dockerfile by copying `Dockerfile.podvm_builder` to `Dockerfile.podvm_builder.DISTRO` and
   adjusting the file properly (e.g. replace `FROM ubuntu:24.04` with `FROM DISTRO`). Try to keep the same
   software versions (e.g. Golang) as much as possible.
2. Create the podvm image dockerfile by copying `Dockerfile.podvm` to `Dockerfile.podvm.DISTRO` and adjusting the file
   properly likewise. In particular, the *PODVM_DISTRO* and *BUILDER_IMG* arguments should be changed.
3. Create the podvm binaries dockerfile by copying `Dockerfile.podvm_binaries`
   to `Dockerfile.podvm_binaries.DISTRO` and adjusting the file as needed.
4. Create the packer directory (`mkdir qcow2/DISTRO`) where the
   `qemu-DISTRO.pkr.hcl` and `variables.pkr.hcl` files should be placed. Also on
   this step you can also use an existing configuration (e.g. `qcow2/ubuntu`) as a
   template. Ensure that common scripts and files like the
   `qcow2/misc-settings.sh` are adjusted to support the DISTRO if needed.
5. Define the base image URL and checksum in the `Makefile` file.
6. Update this *README.md* properly in case that there are specific instructions and/or constraints for the DISTRO.
