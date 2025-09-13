# Cloud API Adaptor (CAA) Documentation

## Overview

The Cloud API Adaptor is a component that enables Kata Pod creation in different cloud providers.
Kata containers runtime on receiving a pod creation request, forwards it to CAA which then creates
a VM in the cloud and runs pod in it. We call this VM a pod VM.
The communication between CAA and the pod VM is over TCP. Any configuration to be provided to the pod VM
before start is passed via user data and processed by the process-user-data service inside the pod VM


## Code Structure

The project is organized as follows:

```
cloud-api-adaptor/
├── cmd/                      # Command-line entry points
│   ├── cloud-api-adaptor/          # Main service entry point. Runs in the Kubernetes worker node
│   └── process-user-data/          # Processes config data made available via user-data inside the pod VM
│   └── agent-protocol-forwarder/   # Service inside the pod VM listening for commands from cloud-api-adaptor (CAA) running in the worker node
├── pkg/
│   ├── adaptor/              # Core adaptor functionality
│   ├── api/                  # API definitions
│   ├── util/                 # Utility functions
│   └── forwarder/            # Network forwarder
├── cloud-providers/          # Cloud-specific implementations
│   ├── aws/                  # AWS provider
│   ├── azure/                # Azure provider
│   ├── ibmcloud/             # IBM Cloud provider
│   ├── libvirt/              # Libvirt provider
│   └── vsphere/              # VMware vSphere provider
└── podvm/                    # Pod VM related code
|   ├── qcow2/                # QCOW2 image definitions for use with packer (deprecated)
|   └── files/                # Systemd units, binaries to be placed in the pod VM image
└── podvm-mkosi/              # Mkosi image building recipe and config filesPod VM related code
    
```

## mkosi Image Building Process

mkosi (Make Operating System Image) is used to build the pod VM images.
A raw read-only and verity protected image is created. This is then booted in the cloud provider.
The container images are downloaded and stored in memory backed file system (tmpfs)

The process involves:

1. **Configuration Setup**: 
   - Configuration files are stored in `podvm-mkosi`
   - The systemd units and executables are copied from `podvm/files`
   - The executable binaries are copied to podvm-mkosi as part of executing `podvm/Dockerfile.podvm_binaries.fedora`



## Contribution Guidelines

1. Follow the Go coding standards
2. Add tests for new functionality
3. Update documentation for significant changes
