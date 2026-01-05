# Introduction

This repository contains the implementation of Kata Containers'
[remote hypervisor interface](https://github.com/kata-containers/kata-containers/blob/main/src/runtime/virtcontainers/remote.go).
Kata remote hypervisor enables creation of Kata VMs on any environment without requiring baremetal servers or nested
virtualization support.

## Goals

* Accept requests from Kata shim to create/delete Kata VM instances without requiring nested virtualization support.
* Manage VM instances in the cloud to run pods using cloud (virtualization) provider APIs
* Forward communication between kata shim on a worker node VM and kata agent on a pod VM
* Provide a mechanism to establish a network tunnel between a worker and pod VMs to Kubernetes pod network

## Components

* Cloud API adaptor ([cmd/cloud-api-adaptor](./cmd/cloud-api-adaptor)) - `cloud-api-adator` implements the remote hypervisor support.
* Agent protocol forwarder ([cmd/agent-protocol-forwarder](./cmd/agent-protocol-forwarder))

## Installation

Please refer to the instructions mentioned in the following [doc](./install/README.md).

## Supported Providers

* aws
* azure
* ibmcloud
* libvirt

### Adding a new provider

Please refer to the instructions mentioned in the following [doc](./docs/addnewprovider.md).

## Cloud Provider VM Image

A custom VM image, which contains the required components, must be available in your cloud provider's image catalogue. You can find detailed instructions for
each provider in their respective directories. You can also find further information in the podvm [README.md](./podvm/README.md) about how to build your own
image using Docker to build the required components and create the image.

> At time of writing the project is moving towards using [mkosi](https://github.com/systemd/mkosi) as our build approach, more information on this can be found
> in the podvm-mkosi [README.md](./podvm-mkosi/README.md).

### VM Image Build Quick Start

To create a QCOW2 image which can be imported into your provider of choice, you can use the following command.

```bash
# default ubuntu based, x86 architecture image
make podvm-builder podvm-binaries podvm-image
# or to produce an s390x architecture image
ARCH=s390x make podvm-builder podvm-binaries podvm-image
# or to produce a rhel distribution image
PODVM_DISTRO=rhel IMAGE_URL=/path/to/rhel.qcow2 make podvm-builder podvm-binaries podvm-image
```

> N.B. This will populate the image using the component versions found in [versions.yaml](./versions.yaml) (for RHEL image download it and set the path as the IMAGE_URL).

You can find provider specific instructions on how to import the QCOW2 image for each cloud provider in their respective directories.
