# Support & Maintenance Levels for Cloud API Adaptor Features

## Introduction

Maintainers and contributors are vital to any open-source project's vitality and the same is true for the
Cloud API Adaptor (CAA). Within the project, we have code for many cloud providers, features, and platforms,
but not all of them have the same level of support. This document aims to outline the maintenance levels of
components in CAA, to help users understand the classification of each component.

## Levels of Support

We have five categories of maintenance/support:

1. Supported: Actively maintained by assigned or paid contributor(s), has updated documentation, upstream CI
   to test behaviour and ensure it stays stable
1. Best effort: Either limited availability from maintainers, lack of support on certain environments,
   missing/outdated documentation, or lacking CI tests to ensure its stability
1. Not maintained: No current maintainers, but the community is open to receive them. May be moved into deprecated
1. Experimental: A new(ish) feature that hasn't reached maturity yet. Not recommended for use
1. Deprecated: Deprecated and unsupported. Recommended to move away from it. Might have a date/release target for removal

## Becoming a maintainer

To become a maintainer, first become a committer and then reach out on the
[#confidential-containers-peerpod](https://cloud-native.slack.com/archives/C04A2EJ70BX) Slack channel or
attend the every other week [community meeting](https://docs.google.com/document/d/1QtiOpSavz177Nq3jwzjQ5vvZjquOT7tFs9XBplNeC-o)
(14:00–15:00 UTC on Wednesdays). Maintainers are tracked via
[@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers), or for specific
features - by the listing below.

## Support classification

| "Feature" | Support State | Maintainers | Notes |
| --- | --- | --- | --- |
| **Cloud Providers** | | | |
| libvirt | Supported | [@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers) | Daily e2e CI on amd64 and s390x. Primary provider for local development and testing. |
| Azure | Supported |  | Daily e2e CI (schedule/dispatch only, not on PRs). Supports CVMs with vTPM. |
| BYOM (Bring Your Own Machine) | Supported | @bpradipt, @Amulyam24 | Daily e2e CI. Uses pre-created VMs via SSH/SFTP.|
| Alibaba Cloud | Best effort |  | Daily e2e CI, but no dedicated maintainers. |
| AWS | No maintainers |  | Limited development/maintenance. Daily e2e CI. Works with non-TEE and AMD SEV-SNP instances. |
| IBM Cloud PowerVS | Not maintained |  | No upstream CI. Documentation exists. |
| GCP | Not maintained | | No upstream CI. No maintainers. Documentation exists. Works with SEV-SNP and TDX instances |
| IBM Cloud VPC | Not maintained |  | No upstream CI.  No maintainers. Documentation exists. |
| | | | |
| **Architectures** | | | |
| amd64 | Supported | [@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers) | Tested in CI across multiple providers |
| s390x | Supported | @stevenhorsman, @BbolroC | Supported for libvirt provider. CAA image and podvm built and pushed for s390x. |
| ppc64le | Best effort | @Amulyam24 | CAA image built and pushed in CI. No e2e tests. |
| arm64 | Not maintained | | No CI, no active development |
| | | | |
| **TEE Platforms** | | | |
| Azure CVM with vTPM | Supported |  | Dedicated podvm image built in CI via `az-cvm-vtpm` TEE platform |
| IBM Secure Execution (s390x) | Best effort | @stevenhorsman, @BbolroC | Supported via libvirt/s390x and IBM Cloud VPC. No upstream TEE CI. |
| AMD SEV-SNP (AWS) | Not maintained |  | CI exists via AWS e2e tests but TEE-specific validation is limited |
| GCP TDX / SEV / SEV-SNP | Not maintained | | Supported via GCP provider `ConfidentialType` config. No upstream CI. No maintainers. |
| | | | |
| **Core Components** | | | |
| cloud-api-adaptor (daemon) | Supported | [@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers) | Core component. Built and tested in CI across amd64 and s390x. |
| agent-protocol-forwarder | Supported | [@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers) | Part of the podvm image. Bridges shim ↔ kata-agent over the network. |
| peerpod-ctrl | Supported | [@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers) | Built and pushed in CI. Handles dangling resource cleanup. |
| webhook | Best effort | | No dedicated maintainers, or active development. Built and pushed in CI. Required for peer pod resource mutation. |
| | | | |
| **Pod VM Image** | | | |
| mkosi-based podvm (amd64) | Supported | [@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers) | Built in CI on every daily run and on PRs |
| mkosi-based podvm (s390x) | Supported | @stevenhorsman, @BbolroC | Built in CI on every daily run and on PRs |
| mkosi-based podvm (Azure/CVM) | Supported |  | Separate image built for `az-cvm-vtpm` TEE platform |
| | | | |
| **Container Runtime Integrations** | | | |
| containerd | Supported | [@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers) | Primary runtime. Used across all provider e2e tests. |
| CRI-O | Best effort | | Used in AWS e2e tests. Limited coverage beyond that. |
| | | | |
| **Deployment** | | | |
| Helm charts | Supported | [@confidential-containers/peer-pod-maintainers](https://github.com/orgs/confidential-containers/teams/peer-pod-maintainers) | Primary deployment method. Tested in CI. |
