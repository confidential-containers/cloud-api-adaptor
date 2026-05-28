# Consuming pre-built podvm images

The project has automation in place to build container images that contain the pod vm qcow2 file. Those images are re-built for each release of this project.
You can use the qcow2 to create provider specific image (eg. AWS AMI or Azure VM image etc.).

Pod VM images are generated using mkosi.

>Note: The published images doesn't have any TEE support. For specific TEE support, you'll need to build your own pod VM image or used the published Azure, AWS and GCP images.
Refer to the coco [website](https://confidentialcontainers.org/docs/examples/) for more details.

You can find the mkosi images available at <https://quay.io/organization/confidential-containers> with the *podvm-generic-ubuntu-[ARCH]* name pattern.

For example:

- <https://quay.io/repository/confidential-containers/podvm-generic-ubuntu-amd64> hosts the Ubuntu images that can be used with all providers.
- <https://quay.io/repository/confidential-containers/podvm-generic-ubuntu-s390x> hosts the Ubuntu images that can be used for s390x architecture.

## Downloading the mkosi based images

The mkosi based images are OCI artifacts, so you'll need to use [oras](https://oras.land/docs/installation/) to download the image

```sh
export CAA_VERSION=v0.13.0
oras pull quay.io/confidential-containers/podvm-generic-fedora-amd64:$CAA_VERSION
tar xvJpf podvm.tar.xz
```
