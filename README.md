# Introduction

This repository contains the implementation of Kata [remote hypervisor](https://github.com/kata-containers/kata-containers/tree/CCv0).
The remote hypervisor concept is currently work in progress. The primary purpose of remote hypervisor support is to create
Kata VMs alongside the Kubernetes worker node VMs, without requiring any nested virtualization support.

## Goals

* Accept requests from Kata shim to create/delete Kata VM instances without requiring nested virtualization support.
* Manage VM instances in the cloud to run PODs using cloud (virtualization) provider APIs
* Forward communication between kata shim on a worker node VM and kata agent on a pod VM
* Provide a mechanism to establish a network tunnel between a worker and pod VMs to Kubernetes pod network

## Architecture

The high level architecture is described in the picture below
![Architecture](./docs/architecture.png)

## Components

* Cloud API adaptor ([cmd/cloud-api-adaptor](./cmd/cloud-api-adaptor)) - `cloud-api-adator` implements the remote hypervisor support.
* Agent protocol forwarder ([cmd/agent-protocol-forwarder](./cmd/agent-protocol-forwarder))
* A modified version of Kata containers to support remote hypervisor based on CCv0 branch (not included in this repository)

## Installation

Please following the instructions mentioned in the following [doc](install/README.md).

## Contribution

This project uses [the Apache 2.0 license](./LICENSE). Contribution to this project requires the [DCO 1.1](./DCO1.1.txt) process to be followed.
