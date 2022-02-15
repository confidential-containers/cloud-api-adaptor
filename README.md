# Cloud API adaptor for Peer Pod VMs

This repository contains the core components for Peer Pod VMs.
A goal of this project is to secure workload pods from Kubernetes administrators by running them in a separate VM from a worker node VM.

## Goals

* Accept requests from Kata shim to create/delete cloud VM instances
* Manage VM instances to run pods using cloud API endpoint
* Forward communication between kata shim on a worker node VM and kata agent on a pod VM
* Provide a mechanism to establish a network tunnel between a worker and pod VMs to Kubernetes pod network

## Architecture

Architecture document is coming soon...

![Architecture](./docs/architecture.png)

## Components

* Cloud API adaptor ([cmd/cloud-api-adaptor](./cmd/cloud-api-adaptor))
* Agent protocol forwarder ([cmd/agent-protocol-forwarder](./cmd/agent-protocol-forwarder))
* A modified version of the shim of Kata containers CCv0 (not included in this repository)

## Contribution

This project uses [the Apache 2.0 license](./LICENSE). Contribution to this project requires the [DCO 1.1](./DCO1.1.txt) process to be followed.
