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

> The podvm build uses [mkosi](https://github.com/systemd/mkosi); see the podvm [README.md](./podvm/README.md) for details.

### VM Image Build Quick Start

To create a bootable image which can be imported into your provider of choice, you can use the mkosi-based build system in the `podvm/` directory.

```bash
# Build Ubuntu-based image
cd podvm
make  # builds builder, binaries, and OS image

# Build with specific TEE platform support (e.g., SNP)
cd podvm
TEE_PLATFORM=snp make image

# Convert to QCOW2 format for libvirt
qemu-img convert -f raw -O qcow2 build/system.raw build/system.qcow2
```

> N.B. This will populate the image using the component versions found in [versions.yaml](./versions.yaml). See [podvm/README.md](./podvm/README.md) for detailed build instructions and customization options.

You can find provider specific instructions on how to import the QCOW2 image for each cloud provider in their respective directories.
