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
- The kata-containers source branch that we use in the [podvm `Dockerfiles`](../podvm/) and the
[podvm workflows](../../../.github/workflows), by updating the `git.kata-containers.reference` value in [versions.yaml](../versions.yaml) to
the tag of the kata-containers release candidate.
- The `kata-containers/src/runtime` go module that we include in the main `cloud-api-adaptor` [`go.mod`](../go.mod) and the `csi-wrapper` [`go.mod`](../../csi-wrapper/go.mod).
This can be done by running
    ```
    go get github.com/kata-containers/kata-containers/src/runtime@<release candidate branch e.g. CCv0>
    go mod tidy
    ```
in the [cloud-api-adaptor](../) directory and [csi-wrapper](../../csi-wrapper/) directory.

- The attestation-agent that is built into the peer pod vm image, by updating the `git.guest-components.reference` value in [versions.yaml](../versions.yaml)

Please ensure podvm image build don't break:
- build podvm image manually
- or use the GHA [Publish pod VM Images](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/.github/workflows/podvm_publish.yaml)

These updates should be done in a PR that is merged triggering the [project images publish workflow](../../../.github/workflows/publish_images_on_push.yaml) to create a new container image in
[`quay.io/confidential-containers/cloud-api-adaptor`](https://quay.io/repository/confidential-containers/cloud-api-adaptor?tab=tags) to use in testing.

#### Tags and update go submodules

As mentioned above we have some go submodules with dependencies in the cloud-api-adaptor repo, so in order to allow
people to use `go get` on these submodules, we need to ensure we create tags for each of the go modules we have in
the correct order.

> [!IMPORTANT]\
> After a tag has been set, it cannot be moved!
> The Go module proxy caches the hash of the first tag and will refuse any update.
> If you mess up, you need to restart the tagging with the next patch version.

The process should go something like:
- Get the release candidate version: `v<version>-alpha.1` (e.g. `v0.8.0-alpha.1` for the confidential containers `0.8.0` release release candidate).

- Update the [peerpod-ctrl go module](../../peerpod-ctrl/go.mod) to use the release candidate version version of `cloud-providers`
- Update the [cloud-api-adaptor go module](../../cloud-api-adaptor/go.mod) to use the release candidate version version of `cloud-providers` and `peerpod-ctrl`
- Update the [csi-wrapper go module](../../csi-wrapper/go.mod) to use the the release candidate version version of `cloud-api-adaptor`

Please keep the local replace references for `cloud-providers`, `peerpod-ctrl` and `cloud-api-adaptor`
and run `make tidy` under the [cloud-api-adaptor](../) to update packages for each go modules.

- Merge the PR with this update to update the `main` branch

- Create a tag for the `cloud-providers` submodule with the name `src/cloud-providers/v<version>-alpha.1`:
    ```bash
    git tag src/cloud-providers/v<version>-alpha.1 main
    git push origin src/cloud-providers/v<version>-alpha.1
    ```
- Create a tag for the `peerpod-ctrl` submodule with the name `src/peerpod-ctrl/v<version>-alpha.1`:
    ```bash
    git tag src/peerpod-ctrl/v<version>-alpha.1 main
    git push origin src/peerpod-ctrl/v<version>-alpha.1
    ```
- Create a tag for the `cloud-api-adaptor` submodule with the name `src/cloud-api-adaptor/v<version>-alpha.1`:
    ```bash
    git tag src/cloud-api-adaptor/v<version>-alpha.1 main
    git push origin src/cloud-api-adaptor/v<version>-alpha.1
    ```
- Create a tag for the `peerpodconfig-ctrl` submodule with the name `src/peerpodconfig-ctrl/v<version>-alpha.1`:
    ```bash
    git tag src/peerpodconfig-ctrl/v<version>-alpha.1 main
    git push origin src/peerpodconfig-ctrl/v<version>-alpha.1
    ```
- Create a tag for the `csi-wrapper` submodule with the name `src/csi-wrapper/v<version>-alpha.1`:
    ```bash
    git tag src/csi-wrapper/v<version>-alpha.1 main
    git push origin src/csi-wrapper/v<version>-alpha.1
    ```
- Create a tag for the `webhook` submodule with the name `src/webhook/v<version>-alpha.1`:
    ```bash
    git tag src/webhook/v<version>-alpha.1 main
    git push origin src/webhook/v<version>-alpha.1
    ```
- After this we should create a cloud-api-adaptor [pre-release](https://github.com/confidential-containers/cloud-api-adaptor/releases/new)
named `v<version>-alpha.1` to trigger the creation of the podvm build.

These versions should be tested to ensure that there are no breaking changes and the wider confidential-containers
release team updated with the status. If there are any issues then this phase might be repeated until it is
successful.

### Cutting releases

During this phase the successful release candidates commits are used to cut proper releases for all the components
and then the projects that use them updated to point to these releases and re-tested. This shouldn't introduce any
instability and all these versions where tested in the release candidate testing phase.

For the cloud-api-adaptor we need to wait until the Kata Containers release tag has been created and the
[Kata Containers runtime payload](https://github.com/kata-containers/kata-containers/actions/workflows/cc-payload.yaml)
to have been built.
Also we need to wait until the [CoCo operator](https://github.com/confidential-containers/operator/) release tag has been create to pin the URLs used by the make `deploy` target to install the operator. So edit the [Makefile](../Makefile) to replace the *github.com/confidential-containers/operator/config/default* and *github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods* URLs, e.g.:
```
sed -i 's#\(github.com/confidential-containers/operator/config/release\)#\1?ref=v0.8.0#' Makefile
sed -i 's#\(github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods\)#\1?ref=v0.8.0#' Makefile
```

Once this has been completed and merged in we should pin the cloud-api-adaptor image used on the deployment files. You should use the commit SHA-1 of the last built `quay.io/confidentil-containers/cloud-api-image` image to update the overlays kustomization files. For example, suppose the release image is `quay.io/confidential-containers/cloud-api-adaptor:6d7d2a3fe8243809b3c3a710792c8498292e2fc3`:
```
cd install/overlays/
for p in aws azure ibmcloud ibmcloud-powervs vsphere; do cd aws; kustomize edit set image cloud-api-adaptor=quay.io/confidential-containers/cloud-api-adaptor:6d7d2a3fe8243809b3c3a710792c8498292e2fc3; cd -; done

# Note that the libvirt use the tag with prefix 'dev-'
cd libvirt; kustomize edit set image cloud-api-adaptor=quay.io/confidential-containers/cloud-api-adaptor:dev-6d7d2a3fe8243809b3c3a710792c8498292e2fc3; cd -
```
After these version updates have been merged via new PR.
We then can repeat the steps done during the release candidate phase, but this time use the
release tags of the project dependencies e.g. `v0.8.0` and creating the tags without the `-alpha.x` suffix.
```bash
git tag src/cloud-providers/v<version> main
git push origin src/cloud-providers/v<version>

git tag src/peerpod-ctrl/v<version> main
git push origin src/peerpod-ctrl/v<version>

git tag src/cloud-api-adaptor/v<version> main
git push origin src/cloud-api-adaptor/v<version>

git tag src/csi-wrapper/v<version> main
git push origin src/csi-wrapper/v<version>

git tag src/peerpodconfig-ctrl/v<version> main
git push origin src/peerpodconfig-ctrl/v<version>

git tag src/webhook/v<version> main
git push origin src/webhook/v<version>
```
We can run the latest release of the cloud-api-adaptor including the auto generated release notes.

This will trigger the podvm builds to happen again and we should re-test the release code before updating the
confidential-containers release team to let them know it has completed successfully

### Post-release

The CoCo operator URLs on the [Makefile](../Makefile) should be reverted to use the latest version.

The changes on the overlay kustomization files should be reverted to start using the latest cloud-api-adaptor images again:
```
cd install/overlays/
for p in aws azure ibmcloud ibmcloud-powervs libvirt vsphere; do cd aws; kustomize edit set image cloud-api-adaptor=quay.io/confidential-containers/cloud-api-adaptor:latest; cd -; done
```

References to Kata Containers should be reverted to the CCv0 branch in:

* [podvm_builder.yaml workflow](../../../.github/workflows/podvm_builder.yaml)
* [podvm_builder `Dockerfiles`](../podvm/)
* go modules (`cloud-api-adaptor` [`go.mod`](../go.mod), the `peerpod-ctl` [`go.mod`](../../peerpod-ctrl/go.mod) and the `csi-wrapper` [`go.mod`](../../csi-wrapper/go.mod))

The `CITATION.cff` needs to be updated with the dates from the release.

## Improvements

Issues that we have to improve the release process that will impact this doc:

- Build the podvm images on the [release candidate testing](#release-candidate-testing) phase ([Issue #1253](https://github.com/confidential-containers/cloud-api-adaptor/issues/1253))
