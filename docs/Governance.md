# Project Governance

Cloud API Adaptor (CAA) is part of the CNCF Confidential Containers (CoCo) organisation and as such follows the governance model of the CNCF and the guidance of the Confidential Containers Steering Committee (SC).

## Vendor Neutrality

The project's goal is to make CoCo accessible to users of various public and private cloud deployments. Hence, it is inherently vendor-neutral and community-driven. Cloud provider implementations are usually maintained by contributors from the respective cloud vendors or interested parties. Maintenance and evolution of the CAA infrastructure and the reference cloud-provider implementation (libvirt) is a shared responsibility of all contributors.

We track the support status, maturity and limitations of each cloud provider implementation [here](./Maintenance.md).

## Alignment with CoCo

Development is closely aligned with Kata Containers project and siblings in the CoCo org. While CAA can be used as a testing ground for experimental features, those might be reconsidered, deprecated and removed if they cannot be reconciled with the goals of the larger CoCo project.

## Submissions

Pull requests are welcome from anyone. The acceptance of a new cloud-provider implementation or a new subsystem is contingent on the ability of the submitter and the community to meaningfully maintain it. This implies the donation of time and often specific compute resources for continuous integration and regression testing.

A pull request requires at least 2 approvals from individuals in the Committers or Maintainers group.

## Groups

We follow the CNCF Governance Guidance regarding the responsibilities and privileges of the different groups. The following description is limited to technical aspects of project management.

### Committers

This group contains individuals who regularly submit meaningful contributions to the project. They are able to provide feedback on pull-requests that count towards the 2-approval threshold that is required for the merging of a feature. They can merge a pull request and have write-access to the repository. Inclusion in this group is either by invitation or self-nomination.

### Maintainers

This group contains individuals who commit time on a sustained basis to the project to further its development and maintain its quality and security standards. They are usually long-term contributors who have gained an understanding of the project's architecture to assess the long-term impact of suggested features and changes. A maintainer is expected to perform code reviews, provide guidance and carry out various maintenance tasks (e.g. dependency updates, security patches, releases). Their approval on a pull-request counts towards the 2-approval threshold that is required for the merging of a feature. They can merge a pull request and have write-access to the repository.

### Admins

Admin is a technical role that is held by individuals in the Maintainer group. They are able to configure the repository settings and manage the project infrastructure. Admins are expected to be active in the project and the membership to this group should be reviewed on a regular basis.

## Collaboration

Collaboration is happening on the CNCF slack instance in the [#confidential-containers-peerpod](https://cloud-native.slack.com/archives/C04A2EJ70BX) channel. Issues can be raised on the project's GitHub repository. The project holds a bi-weekly community call on [Zoom](https://zoom-lfx.platform.linuxfoundation.org/meeting/98394123719?password=81f308af-8a01-4ee1-857c-9b060fcca144) to discuss ongoing work and to coordinate efforts. The meeting is open to all interested parties and recorded. It occurs from **14:00-15:00 UTC** on **Wednesday** every **uneven week** of the year.

A community-call requires a quorum of at least 3 individuals from the Maintainers or Committers group. The meeting is usually moderated by a Maintainer, who is responsible for the agenda and the meeting notes.
