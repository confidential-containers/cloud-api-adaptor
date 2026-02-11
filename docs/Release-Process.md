# Release process

This document lists how to do a release of 'Peer pods' functionality in the context of a wider Confidential
Containers release

## Release phases

In the new Confidential Containers
[release process](https://github.com/confidential-containers/community/blob/main/.github/ISSUE_TEMPLATE/release-check-list.md),
the plan is to do a succession of component releases, rather than releasing
all components at the same time. This means that the peer pods release process needs to slot into the correct place.
The flow of releases should roughly be:

1. The [guest-components release](https://github.com/confidential-containers/guest-components/releases) (or a pinned
   version is picked) and [trustee releases](https://github.com/confidential-containers/trustee/releases).
    - This triggers [kata-containers](https://github.com/kata-containers/kata-containers) to update to these new versions in
      [versions.yaml](https://github.com/kata-containers/kata-containers/blob/main/versions.yaml) under
      `externals.coco-guest-components.version`, `externals.coco-trustee` and the `image-rs` crate in the agent's
      [`Cargo.toml`](https://github.com/kata-containers/kata-containers/blob/main/src/agent/Cargo.toml).
    - At this point it makes sense for us to stay in sync, by updating the guest-components and kbs that we use in peer pods,
      by changing the `oci.guest-components.reference` and `git.kbs.tag`values in [versions.yaml](../src/cloud-api-adaptor/versions.yaml).
      We should also bump the kata agent to the latest commit hash by updating `oci.kata-containers.reference` in
      [version.yaml](../src/cloud-api-adaptor/versions.yaml) for testing.

1. Kata Containers [releases](https://github.com/kata-containers/kata-containers/releases)
    - We should already be in sync with the guest-components and trustee, from the previous step, but now we should update:
      - The kata-containers source branch that we use in [versions.yaml](../src/cloud-api-adaptor/versions.yaml) to
        the kata-containers release version.
      - The `kata-containers/src/runtime` go module that we include in the main `cloud-api-adaptor` [`go.mod`](../src/cloud-api-adaptor/go.mod) and the `csi-wrapper` [`go.mod`](../src/csi-wrapper/go.mod). This can be done by running
        ```
        go get github.com/kata-containers/kata-containers/src/runtime@<latest release e.g. 3.6.0>
        go mod tidy
        ```
        in the [cloud-api-adaptor](../src/cloud-api-adaptor/) directory and [csi-wrapper](../src/csi-wrapper/) directory.
    - Update the `dependencies.kata-deploy` version in [`Chart.yaml`](../src/cloud-api-adaptor/install/charts/peerpods/Chart.yaml) to the Kata release version.

1. cloud-api-adaptor releases with the following phases detailed below:
    - Cutting the release
    - Helm chart release
    - Post release tasks

### Cutting releases

Once the pre-release versions are tested and stable, then we can go ahead and cut the release of "peer pods".

As part of the release we should pin the cloud-api-adaptor image used on the deployment files. You should use the
commit SHA-1 of the last built `quay.io/confidential-containers/cloud-api-adaptor` image to update the
Helm chart values. For example, suppose the release image is
`quay.io/confidential-containers/cloud-api-adaptor:6d7d2a3fe8243809b3c3a710792c8498292e2fc3`, we can use the
`release-helper.sh` script's `caa-image-tag` command to update all the cloud-providers:

```
RELEASE_TAG="6d7d2a3fe8243809b3c3a710792c8498292e2fc3"
./hack/release-helper.sh caa-image-tag ${RELEASE_TAG}
```

In the same PR, bump the Helm chart versions in
[`Chart.yaml`](../src/cloud-api-adaptor/install/charts/peerpods/Chart.yaml):
- `version`: bump to the new chart release version (e.g. `0.1.0` => `0.1.1`)
- `appVersion`: set to the CAA release version (e.g. `v0.18.0`)

Include those changes within a new PR to the `main` branch.

Make sure to update the local `main` branch after the PR is merged.

From the main branch, create git tags for the release, for all go modules e.g. To push the tags on the `upstream` remote (this remote should point to the `confidential-containers/cloud-api-adaptor` repo) for the `v0.8.0` release, run:

```bash
./hack/release-helper.sh go-tag v0.8.0 upstream
The intput release tag: v0.8.0
The follow git commands can be used to do release tags.
*****************************IMPORTANT********************************************
After a tag has been set, it cannot be moved!
The Go module proxy caches the hash of the first tag and will refuse any update.
If you mess up, you need to restart the tagging with the next patch version.
**********************************************************************************
git tag src/cloud-api-adaptor/v0.8.0 main
git push upstream src/cloud-api-adaptor/v0.8.0
git tag src/cloud-providers/v0.8.0 main
git push upstream src/cloud-providers/v0.8.0
git tag src/csi-wrapper/v0.8.0 main
git push upstream src/csi-wrapper/v0.8.0
git tag src/peerpod-ctrl/v0.8.0 main
git push upstream src/peerpod-ctrl/v0.8.0
git tag src/webhook/v0.8.0 main
git push upstream src/webhook/v0.8.0
```

Copy and paste the generated commands to create and push release tags, the output looks like:

```bash
Total 0 (delta 0), reused 0 (delta 0), pack-reused 0 (from 0)
To github.com:confidential-containers/cloud-api-adaptor.git
 * [new tag]         src/cloud-api-adaptor/v0.8.0 -> src/cloud-api-adaptor/v0.8.0
Total 0 (delta 0), reused 0 (delta 0), pack-reused 0 (from 0)
To github.com:confidential-containers/cloud-api-adaptor.git
 * [new tag]         src/cloud-providers/v0.8.0 -> src/cloud-providers/v0.8.0
Total 0 (delta 0), reused 0 (delta 0), pack-reused 0 (from 0)
To github.com:confidential-containers/cloud-api-adaptor.git
 * [new tag]         src/csi-wrapper/v0.8.0 -> src/csi-wrapper/v0.8.0
Total 0 (delta 0), reused 0 (delta 0), pack-reused 0 (from 0)
To github.com:confidential-containers/cloud-api-adaptor.git
 * [new tag]         src/peerpod-ctrl/v0.8.0 -> src/peerpod-ctrl/v0.8.0
Total 0 (delta 0), reused 0 (delta 0), pack-reused 0 (from 0)
To github.com:confidential-containers/cloud-api-adaptor.git
 * [new tag]         src/webhook/v0.8.0 -> src/webhook/v0.8.0
```

We then create a cloud-api-adaptor [release](https://github.com/confidential-containers/cloud-api-adaptor/releases/new)
named `v<version>`. Choose the "Create new tag" option when drafting a release.

This will trigger the podvm builds to happen again and we should re-test the release code before updating the
confidential-containers release team to let them know it has completed successfully

### Helm chart release

After the GitHub release is created and the release workflow has finished building the images, we can publish the
Helm chart. Trigger the
[`publish-peerpods-chart`](../.github/workflows/publish-peerpods-chart.yaml) workflow manually from the `main`
branch. This will package the chart and push it to the OCI registry at
`oci://ghcr.io/confidential-containers/cloud-api-adaptor/charts/peerpods`.

### Post-release

If the `main` branch was not already unlocked, then ask an admin to do this now.

Update the `dependencies.kata-deploy` version in [`Chart.yaml`](../src/cloud-api-adaptor/install/charts/peerpods/Chart.yaml)  should be reverted to use the `0.0.0-dev` tag.

Revert `image.tag` back to `"latest"` in
[`values.yaml`](../src/cloud-api-adaptor/install/charts/peerpods/values.yaml),
[`providers/libvirt.yaml`](../src/cloud-api-adaptor/install/charts/peerpods/providers/libvirt.yaml), and
[`providers/docker.yaml`](../src/cloud-api-adaptor/install/charts/peerpods/providers/docker.yaml).

Update strings in documentation (e.g. `0.7.0` => `0.8.0`) and the `CITATION.cff` file with the release date, git sha and version.
