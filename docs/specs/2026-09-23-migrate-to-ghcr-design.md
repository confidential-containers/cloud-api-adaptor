# Migrate from quay.io to ghcr.io — Design

**Goal:** Remove all image pushes to `quay.io/confidential-containers` from CI workflows and scripts, replacing them with `ghcr.io/confidential-containers` as the sole push destination.

## Approach

The project already pushes every built image to `ghcr.io/confidential-containers`. The quay.io push is either a secondary copy step or a legacy default. The migration is mechanical: make `ghcr.io` the sole push destination, update every default registry value, remove quay-specific login steps and copy jobs, and clean up now-orphaned `QUAY_PASSWORD` secret references.

The work is grouped into four layers:

1. **GitHub Actions workflows** — change defaults, remove quay login steps, remove quay-only jobs and conditions.
2. **Makefiles and build scripts** — change default `REGISTRY`/`IMG` values.
3. **Helm chart values and Dockerfile ARGs** — update default image refs.
4. **Documentation, test code, and scripts** — update image URLs.

## Non-goals

- Migrating third-party test fixture images that are only *pulled* (never pushed by this repo): `quay.io/prometheus/busybox`, `quay.io/nginx/nginx-unprivileged`, `quay.io/curl/curl`. These stay as-is.
- Migrating `quay.io/confidential-containers/test/nginx` — not pushed by this repo's CI; left as-is.
- Migrating `src/cloud-api-adaptor/podvm/Dockerfile.podvm_byom_docker_provider` base image (`quay.io/confidential-containers/podvm-docker-image`) — legacy image outside the current push pipeline.
- Setting up GHCR org-level image visibility or access policies (ops concern).
- Updating consumer documentation in other repositories.

## Additional cleanup

The `csi-podvm-wrapper` image (`quay.io/confidential-containers/csi-podvm-wrapper:latest`) is deprecated and removed. The test code that references it is deleted:
- `NewPodWithPVCFromIBMVPCBlockDriver` function in `ibmcloud_common.go`
- `TestCreatePeerPodWithPVC` test in `ibmcloud_test.go`
- `DoTestCreatePeerPodWithPVCAndCSIWrapper` helper in `common_suite.go`

## Files Changed

### GitHub Actions Workflows

| File | Change |
|---|---|
| `.github/workflows/caa_build_and_push.yaml` | Change default registry to `ghcr.io/confidential-containers`; remove quay login step; drop `QUAY_PASSWORD` secret |
| `.github/workflows/caa_build_and_push_all_arches.yaml` | Change default registry; remove quay login step in manifest job; drop `QUAY_PASSWORD` secret |
| `.github/workflows/peerpod-ctrl_build_and_push.yaml` | Change default registry; remove quay login step; drop `QUAY_PASSWORD` secret |
| `.github/workflows/peerpod-ctrl_build_and_push_all_arches.yaml` | Change default registry; remove quay login step in manifest job; drop `QUAY_PASSWORD` secret |
| `.github/workflows/webhook_image_publish.yaml` | Remove `push-to-quay` job; drop `QUAY_PASSWORD` secret |
| `.github/workflows/podvm_publish.yaml` | Remove `push-to-quay` job; drop `QUAY_PASSWORD` secret |
| `.github/workflows/podvm_byom_binaries_publish.yaml` | Replace quay login with ghcr login; change default registry to `ghcr.io/confidential-containers`; drop `QUAY_PASSWORD` secret |
| `.github/workflows/publish_images_on_push.yaml` | Simplify registry expressions to always use `ghcr.io/{owner}`; remove `QUAY_PASSWORD` secret references |
| `.github/workflows/release.yaml` | Change webhook job's hardcoded `registry: quay.io/confidential-containers` to `ghcr.io/confidential-containers`; remove `QUAY_PASSWORD` secret references |
| `.github/workflows/test-images.yaml` | Replace quay login with ghcr login; push to `ghcr.io/confidential-containers/test-images` |
| `.github/workflows/manual-test-helm.yaml` | Update hardcoded `quay.io` image defaults to `ghcr.io` |
| `.github/workflows/e2e_byom.yaml` | Update `BYOM_PODVM_IMAGE` ref from `quay.io` to `ghcr.io` |
| `.github/workflows/e2e_run_all.yaml` | Remove `QUAY_PASSWORD` secret references |
| `.github/workflows/daily-e2e-tests.yaml` | Remove `QUAY_PASSWORD` secret reference |
| `.github/workflows/e2e_on_pull.yaml` | Remove `QUAY_PASSWORD` secret reference |

### Build Scripts and Makefiles

| File | Change |
|---|---|
| `src/cloud-api-adaptor/hack/build.sh` | Change default `registry` to `ghcr.io/confidential-containers` |
| `src/cloud-api-adaptor/podvm/Makefile` | Change default `REGISTRY` to `ghcr.io/confidential-containers` |
| `src/peerpod-ctrl/Makefile` | Change default `IMG` to `ghcr.io/confidential-containers/peerpod-ctrl:latest` |
| `src/webhook/Makefile` | Change default `IMG` to `ghcr.io/confidential-containers/peer-pods-webhook:latest` |

### Dockerfiles

| File | Change |
|---|---|
| `src/cloud-api-adaptor/podvm/Dockerfile.podvm_byom_binaries` | Change `BINARIES_IMG` ARG default to `ghcr.io/confidential-containers/podvm-binaries-ubuntu-amd64` |

### Helm Charts and Values

| File | Change |
|---|---|
| `src/cloud-api-adaptor/install/charts/peerpods/values.yaml` | Change default `image.name` to `ghcr.io/confidential-containers/cloud-api-adaptor` |
| `src/cloud-api-adaptor/install/charts/peerpods/templates/daemonset.yaml` | Update fallback default image ref |
| `src/peerpod-ctrl/chart/values.yaml` | Change `image.repository` to `ghcr.io/confidential-containers/peerpod-ctrl` |
| `src/webhook/chart/values.yaml` | Change `image.repository` to `ghcr.io/confidential-containers/peer-pods-webhook` |

### Documentation and Scripts

| File | Change |
|---|---|
| `hack/release-helper.sh` | Update text references from `quay.io` to `ghcr.io` |
| `src/cloud-api-adaptor/docs/consuming-prebuilt-podvm-images.md` | Update image pull URLs |
| `src/cloud-api-adaptor/docs/addnewprovider.md` | Update example docker tag |
| `src/cloud-api-adaptor/ibmcloud/IMPORT_PODVM_TO_VPC.md` | Update `oras pull` example URLs |
| `src/webhook/docs/INSTALL.md` | Update `make kind-deploy` example image ref |
| `src/webhook/hack/webhook-deploy.yaml` | Update `image:` ref |

### E2E Test Code

| File | Change |
|---|---|
| `src/cloud-api-adaptor/test/e2e/versions.yaml` | Update `registry` for `curl`/`test-images` entry from `quay.io/confidential-containers` to `ghcr.io/confidential-containers` |
| `src/cloud-api-adaptor/test/e2e/common_suite.go` | Update hardcoded `quay.io/confidential-containers/test-images:*` refs to `ghcr.io/confidential-containers/test-images:*`; **delete** `DoTestCreatePeerPodWithPVCAndCSIWrapper` |
| `src/cloud-api-adaptor/test/e2e/ibmcloud_common.go` | **Delete** `NewPodWithPVCFromIBMVPCBlockDriver` function and its now-orphaned imports (`corev1`, `intstr`, `ptr`) |
| `src/cloud-api-adaptor/test/e2e/ibmcloud_test.go` | **Delete** `TestCreatePeerPodWithPVC` and its now-orphaned import (`corev1`) |
