[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fconfidential-containers%2Fcloud-api-adaptor.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fconfidential-containers%2Fcloud-api-adaptor?ref=badge_shield&issueType=license)
[![daily e2e tests for libvirt](https://github.com/confidential-containers/cloud-api-adaptor/actions/workflows/daily-e2e-tests-libvirt.yaml/badge.svg)](https://github.com/confidential-containers/cloud-api-adaptor/actions/workflows/daily-e2e-tests-libvirt.yaml)
[![nightly build for azure](https://github.com/confidential-containers/cloud-api-adaptor/actions/workflows/azure-nightly-build.yml/badge.svg)](https://github.com/confidential-containers/cloud-api-adaptor/actions/workflows/azure-nightly-build.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/confidential-containers/cloud-api-adaptor/badge)](https://scorecard.dev/viewer/?uri=github.com/confidential-containers/cloud-api-adaptor)

# Introduction

This repository contains all go modules related to Cloud API Adaptor. The Cloud API Adaptor is an implementation of the
[remote hypervisor interface](https://github.com/kata-containers/kata-containers/blob/main/src/runtime/virtcontainers/remote.go)
of [Kata Containers](https://github.com/kata-containers/kata-containers)

It enables the creation of Kata Containers VMs on any machines without the need for bare metal worker nodes,
or nested virtualisation support.

## Architecture

The background and description of the components involved in 'peer pods' can be found in the [architecture documentation](./docs/architecture.md).

## Cloud Providers
[cloud-providers](./src/cloud-providers/) Cloud Providers for Kata remote hypervisor

## PeerPod controller
[peerpod-ctrl](./src/peerpod-ctrl/) PeerPod controller is watching PeerPod events and deleting dangling resources that were not deleted by the cloud-api-adaptor at Pod deletion time.

## Cloud API Adaptor
[cloud-api-adaptor](./src/cloud-api-adaptor/) Ability to create Kata pods using cloud provider APIs aka the peer-pods approach

## CSI Wrapper
[csi-wrapper](./src/csi-wrapper/) CSI Wrapper solution for Peer Pod Storage

## Webhook
[webhook](./src/webhook/) This mutating webhook modifies a POD spec using specific runtimeclass to remove all `resources` entries and replace it with peer-pod extended resource.

## Contribution

This project uses [the Apache 2.0 license](./LICENSE). Contribution to this project requires the [DCO 1.1](./DCO1.1.txt) process to be followed.

## Collaborations

* Slack: [#confidential-containers-peerpod](https://cloud-native.slack.com/archives/C04A2EJ70BX) in [CNCF](https://communityinviter.com/apps/cloud-native/cncf)
* Zoom meeting: https://zoom.us/j/94601737867?pwd=MEF5NkN5ZkRDcUtCV09SQllMWWtzUT09
    * 14:00 - 15:00 UTC on each `Wednesday`
