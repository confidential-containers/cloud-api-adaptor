# Release process

This document lists how to do a release of 'Peer pods' functionality in the context of a wider Confidential
Containers release

## Release phases

The confidential-containers
[release process](https://github.com/confidential-containers/community/blob/main/.github/ISSUE_TEMPLATE/release-check-list.md)
lists the tasks involved in doing a release and these largely break down to three phases:
- Release candidate selection and testing
- Cutting releases
- Post release

### Release candidate testing

In the release candidate selection and testing phase, we ensure that the dependencies we have within the
confidential-containers projects are updated and that Kata Containers is updated to use these new versions, the
[Kata Containers Runtime Payload CI](https://github.com/kata-containers/kata-containers/actions/workflows/cc-payload-after-push.yaml)
image is updated, the operator is updated and the tests pass with all of these. 

At this point, we should update the cloud-api-adaptor versions to use these release candidate versions of:
- The kata-containers source branch that we use in the [podvm_builder `Dockerfiles`](../podvm/) and the
[`podvm_builder.yaml` workflow](../.github/workflows/podvm_builder.yaml), by updating the `KATA_SRC_BRANCH` value to
the tag of the kata-containers release candidate.
- The `kata-containers/src/runtime` go module that we include in the main `cloud-api-adaptor` [`go.mod`](../go.mod),
the `peerpod-ctl` [`go.mod`](../peerpod-ctrl/go.mod) and the `csi-wrapper` [`go.mod`](../volumes/csi-wrapper/go.mod).
This can be done by running
  ```
  go get github.com/kata-containers/kata-containers/src/runtime@<release candidate branch e.g. CCv0>
  go mod tidy
  ```
in the top-level repo directory, and the `peerpod-ctl` and `volumes/csi-wrapper` directories.
> **Note:** If there are API changes in the kata-runtime go modules and we need to cloud-api-adaptor to implement,
then it may be necessary to temporarily get the peerpod-ctrl and csi-wrapper to self-reference the parent folder to
avoid compilation errors. This can be done by running 
> ```
> go mod edit -replace github.com/confidential-containers/cloud-api-adaptor=../
> go mod tidy
> ```
> from in the `peerpod-ctrl` and `volumes/csi-wrapper` directories.
- The attestation-agent that is built into the peer pod vm image, by updating the `AA_VERSION` in the 
[`Makefile.inc`](../podvm/Makefile.inc)

These updates should be done in a PR that is merged triggering the cloud-api-adaptor
[image build workflow](../.github/workflows/image.yaml) to create a new container image in 
[`quay.io](https://quay.io/repository/confidential-containers/cloud-api-adaptor?tab=tags) to use in testing.

We should also create a cloud-api-adaptor [pre-release](https://github.com/confidential-containers/cloud-api-adaptor/releases/new)
to trigger the creation of the podvm build.

These versions should be tested to ensure that there are no breaking changes and the wider confidential-containers
release team updated with the status. If there are any issues then this phase might be repeated until it is
successful.

### Cutting releases

During this phase the successful release candidates commits are used to cut proper releases for all the components
and then the projects that use them updated to point to these releases and re-tested. This shouldn't introduce any
instability and all these versions where tested in the release candidate testing phase.

For the cloud-api-adaptor we need to wait until the Kata Containers release tag has been created and the
[Kata Containers runtime payload](https://github.com/kata-containers/kata-containers/actions/workflows/cc-payload.yaml)
to have been built. We then can repeat the updates done during the release candidate phase, but this time use the
release tags of the projects e.g. `v0.6.0`.

Once this has been completed and merged in we run the latest release of the cloud-api-adaptor including the auto
generated release notes.

This will trigger the podvm builds to happen again and we should re-test the release code before updating the
confidential-containers release team to let them know it has completed successfully

### Post-release

After the release has been cut the `peerpod-ctrl` and `volumes/csi-wrapper` go modules should be updated to remove
any local replace references, and be updated to use the release version of the `cloud-api-adaptor` by running:
  ```
  go get github.com/confidential-containers/cloud-api-adaptor
  go mod tidy
  ```
from in the `peerpod-ctrl` and `volumes/csi-wrapper` directories.

## Improvements

Issues that we have to improve the release process that will impact this doc:

- Create tags for the cloud-api-adaptor and webhook images on release and update the overlays to point to these
versions in the tag [Issue #1109](https://github.com/confidential-containers/cloud-api-adaptor/issues/1109)