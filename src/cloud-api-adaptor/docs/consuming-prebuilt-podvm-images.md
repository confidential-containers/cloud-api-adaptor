# Consuming pre-built podvm images

The project has automation in place to build container images that contain the pod vm qcow2 file. Those images are re-built for each release of this project. 
You can use the qcow2 to create provider specific image (eg. AWS AMI or Azure VM image etc.).

There are two set of images published:

1. Mkosi generated image with Fedora as the base distro
   This is the preferred method currently to generate pod vm images.
2. Packer generated image with Ubuntu as the base distro

>Note: The published images doesn't have any TEE support. For specific TEE support, you'll need to build your own pod VM image or used the published Azure, AWS and GCP images.
Refer to the coco [website](https://confidentialcontainers.org/docs/examples/) for more details.

You can find the packer images available at https://quay.io/organization/confidential-containers with the *podvm-generic-ubuntu-[ARCH]* name pattern.
You can find the mkosi images available at https://quay.io/organization/confidential-containers with the *podvm-generic-fedora-[ARCH]* name pattern.

For example:

- https://quay.io/repository/confidential-containers/podvm-generic-ubuntu-amd64 hosts the Ubuntu images that can be used with all providers.
- https://quay.io/repository/confidential-containers/podvm-generic-ubuntu-s390x hosts the Ubuntu images that can be used for s390x architecture.
- https://quay.io/repository/confidential-containers/podvm-generic-fedora-amd64 hosts the Fedora images that can be used with all providers.
- https://quay.io/repository/confidential-containers/podvm-generic-fedora-s390x hosts the Ubuntu images that can be used for s390x architecture.

## Downloading the packer based images

The easiest way to extract the packer generated qcow2 file from the podvm container image is using the [`download-image.sh`](../podvm/hack/download-image.sh) script. For example, to extract the file from the *podvm-generic-ubuntu-amd64* image:

```sh
$ export CAA_VERSION=v0.13.0
$ ./src/cloud-api-adaptor/podvm/hack/download-image.sh quay.io/confidential-containers/podvm-generic-ubuntu-amd64:$CAA_VERSION . -o podvm.qcow2
```

>Note: images can be checked from https://quay.io/repository/confidential-containers/podvm-generic-ubuntu-amd64?tab=tags, to get the available tags. The `latest` tag is not available.

## Downloading the mkosi based images

The mkosi based images are OCI artifacts, so you'll need to use [oras](https://oras.land/docs/installation/) to download the image

```sh
$ export CAA_VERSION=v0.13.0
$ oras pull quay.io/confidential-containers/podvm-generic-fedora-amd64:$CAA_VERSION
$ tar xvJpf podvm.tar.xz
```
