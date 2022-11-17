# Introduction

This repository contains the implementation of Kata [remote hypervisor](https://github.com/kata-containers/kata-containers/tree/CCv0).
Kata remote hypervisor enables creation of Kata VMs on any environment without requiring baremetal servers or nested 
virtualization support.

## Goals

* Accept requests from Kata shim to create/delete Kata VM instances without requiring nested virtualization support.
* Manage VM instances in the cloud to run pods using cloud (virtualization) provider APIs
* Forward communication between kata shim on a worker node VM and kata agent on a pod VM
* Provide a mechanism to establish a network tunnel between a worker and pod VMs to Kubernetes pod network

## Architecture

The high level architecture is described in the picture below
![Architecture](./docs/architecture.png)

## Components

* Cloud API adaptor ([cmd/cloud-api-adaptor](./cmd/cloud-api-adaptor)) - `cloud-api-adator` implements the remote hypervisor support.
* Agent protocol forwarder ([cmd/agent-protocol-forwarder](./cmd/agent-protocol-forwarder))

## Installation

Please refer to the instructions mentioned in the following [doc](install/README.md).

## Supported Providers

* aws
* azure
* ibmcloud
* libvirt
* vsphere

### Adding a new provider

Please refer to the instructions mentioned in the following [doc](./docs/addnewprovider.md).

## Contribution

This project uses [the Apache 2.0 license](./LICENSE). Contribution to this project requires the [DCO 1.1](./DCO1.1.txt) process to be followed.

## Collaborations

* Slack: [#confidential-containers-peerpod](https://cloud-native.slack.com/archives/C04A2EJ70BX) in [CNCF](https://communityinviter.com/apps/cloud-native/cncf) 
* Zoom meeting: https://zoom.us/j/94601737867?pwd=MEF5NkN5ZkRDcUtCV09SQllMWWtzUT09
    * 8:00 - 9:00 UTC on each `Wednesday`