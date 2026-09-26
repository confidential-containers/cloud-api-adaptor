# Migrate from quay.io to ghcr.io — Implementation Plan

**Goal:** Remove all image pushes to `quay.io/confidential-containers` from CI workflows and scripts, replacing them with `ghcr.io/confidential-containers`.
**Design:** [docs/specs/2026-09-23-migrate-to-ghcr-design.md](2026-09-23-migrate-to-ghcr-design.md)
**Approach:** Mechanical find-and-replace across four layers: GitHub Actions workflows, Makefiles/build scripts, Helm values/Dockerfiles, and docs/test code. Also deletes the deprecated `csi-podvm-wrapper` test.
**Stack:** YAML, Go, Bash, Makefile

---

### Task 1: Delete deprecated csi-podvm-wrapper test code

**Files:**
- Modify: `src/cloud-api-adaptor/test/e2e/ibmcloud_common.go`
- Modify: `src/cloud-api-adaptor/test/e2e/ibmcloud_test.go`
- Modify: `src/cloud-api-adaptor/test/e2e/common_suite.go`

**Steps:**

- [ ] **In `ibmcloud_common.go`: remove `NewPodWithPVCFromIBMVPCBlockDriver` and orphaned imports**

  Delete the entire function `NewPodWithPVCFromIBMVPCBlockDriver` (lines 56–284).
  Remove `corev1 "k8s.io/api/core/v1"`, `"k8s.io/apimachinery/pkg/util/intstr"`, and `"k8s.io/utils/ptr"` from the import block.

- [ ] **In `ibmcloud_test.go`: remove `TestCreatePeerPodWithPVC` and orphaned import**

  Delete the entire function `TestCreatePeerPodWithPVC` (lines 122–147).
  Remove `corev1 "k8s.io/api/core/v1"` from the import block (only used in that test).

- [ ] **In `common_suite.go`: remove `DoTestCreatePeerPodWithPVCAndCSIWrapper`**

  Delete the entire function `DoTestCreatePeerPodWithPVCAndCSIWrapper` (lines 225–243).

- [ ] **Verify compilation**

  ```bash
  cd src/cloud-api-adaptor && go build -tags ibmcloud ./test/e2e/...
  ```
  Expected: compiles with no errors.

- [ ] **Commit**

  ```bash
  git add src/cloud-api-adaptor/test/e2e/ibmcloud_common.go \
          src/cloud-api-adaptor/test/e2e/ibmcloud_test.go \
          src/cloud-api-adaptor/test/e2e/common_suite.go
  git commit -m "test: remove deprecated csi-podvm-wrapper test code"
  ```

**Acceptance criteria:**
- No reference to `csi-podvm-wrapper`, `NewPodWithPVCFromIBMVPCBlockDriver`, or `DoTestCreatePeerPodWithPVCAndCSIWrapper` anywhere in the codebase.
- Package compiles cleanly with `-tags ibmcloud`.

**Verify:** `grep -r "csi-podvm-wrapper\|NewPodWithPVCFromIBMVPCBlockDriver\|DoTestCreatePeerPodWithPVCAndCSIWrapper" src/cloud-api-adaptor/test/e2e/` — expected: no output.

---

### Task 2: Update callable workflow defaults and login steps — CAA

**Files:**
- Modify: `.github/workflows/caa_build_and_push.yaml`
- Modify: `.github/workflows/caa_build_and_push_all_arches.yaml`

**Steps:**

- [ ] **`caa_build_and_push.yaml`: change default registry and remove quay login**

  Change:
  ```yaml
        default: 'quay.io/confidential-containers'
        description: 'Image registry (e.g. "quay.io/confidential-containers") where the built image will be pushed to'
  ```
  To:
  ```yaml
        default: 'ghcr.io/confidential-containers'
        description: 'Image registry (e.g. "ghcr.io/confidential-containers") where the built image will be pushed to'
  ```

  Remove the entire `Login to quay Container Registry` step:
  ```yaml
      - name: Login to quay Container Registry
        if: ${{ startsWith(inputs.registry, 'quay.io') }}
        uses: docker/login-action@...
        with:
          registry: ${{ inputs.registry }}
          username: ${{ vars.QUAY_USERNAME }}
          password: ${{ secrets.QUAY_PASSWORD }}
  ```

  Remove the `QUAY_PASSWORD` secret declaration from `workflow_call.secrets`.

- [ ] **`caa_build_and_push_all_arches.yaml`: change default registry and remove quay login in manifest job**

  Change the default registry from `quay.io/confidential-containers` to `ghcr.io/confidential-containers` in the `workflow_call.inputs` block.

  Remove the `Login to quay Container Registry` step from the `manifest_job`.

  Remove the `QUAY_PASSWORD` secret declaration from `workflow_call.secrets`.

- [ ] **Commit**

  ```bash
  git add .github/workflows/caa_build_and_push.yaml \
          .github/workflows/caa_build_and_push_all_arches.yaml
  git commit -m "ci: migrate CAA build workflows from quay.io to ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io` references remain in either file.
- No `QUAY_PASSWORD` or `QUAY_USERNAME` references remain in either file.

**Verify:** `grep -n "quay\|QUAY" .github/workflows/caa_build_and_push.yaml .github/workflows/caa_build_and_push_all_arches.yaml` — expected: no output.

---

### Task 3: Update callable workflow defaults and login steps — peerpod-ctrl

**Files:**
- Modify: `.github/workflows/peerpod-ctrl_build_and_push.yaml`
- Modify: `.github/workflows/peerpod-ctrl_build_and_push_all_arches.yaml`

**Steps:**

- [ ] **`peerpod-ctrl_build_and_push.yaml`: change default registry and remove quay login**

  Change default from `quay.io/confidential-containers` to `ghcr.io/confidential-containers` in the `workflow_call.inputs.registry` block.
  Update the description string accordingly.

  Remove the `Login to Quay container Registry` step:
  ```yaml
      - name: Login to Quay container Registry
        if: ${{ startsWith(inputs.registry, 'quay.io') }}
        uses: docker/login-action@...
        with:
          registry: quay.io
          username: ${{ vars.QUAY_USERNAME }}
          password: ${{ secrets.QUAY_PASSWORD }}
  ```

  Remove the `QUAY_PASSWORD` secret declaration.

- [ ] **`peerpod-ctrl_build_and_push_all_arches.yaml`: change default registry and remove quay login in manifest job**

  Change default from `quay.io/confidential-containers` to `ghcr.io/confidential-containers`.

  Remove the `Login to Quay Container Registry` step from the `manifest_job`.

  Remove the `QUAY_PASSWORD` secret declaration.

- [ ] **Commit**

  ```bash
  git add .github/workflows/peerpod-ctrl_build_and_push.yaml \
          .github/workflows/peerpod-ctrl_build_and_push_all_arches.yaml
  git commit -m "ci: migrate peerpod-ctrl build workflows from quay.io to ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io` references remain in either file.
- No `QUAY_PASSWORD` or `QUAY_USERNAME` references remain in either file.

**Verify:** `grep -n "quay\|QUAY" .github/workflows/peerpod-ctrl_build_and_push.yaml .github/workflows/peerpod-ctrl_build_and_push_all_arches.yaml` — expected: no output.

---

### Task 4: Remove push-to-quay jobs from webhook and podvm publish workflows

**Files:**
- Modify: `.github/workflows/webhook_image_publish.yaml`
- Modify: `.github/workflows/podvm_publish.yaml`

**Steps:**

- [ ] **`webhook_image_publish.yaml`: remove the `push-to-quay` job**

  Delete the entire `push-to-quay` job block:
  ```yaml
    push-to-quay:
      name: Push webhook image to quay.io
      ...
      if: ${{ startsWith(inputs.registry, 'quay.io') }}
      ...
  ```

  Remove the `QUAY_PASSWORD` secret declaration from `workflow_call.secrets`.

  Remove or update the `registry` input description to remove mention of quay.io.

- [ ] **`podvm_publish.yaml`: remove the `push-to-quay` job**

  Delete the entire `push-to-quay` job block:
  ```yaml
    push-to-quay:
      name: Push podvm image to quay.io
      ...
      if: ${{ startsWith(inputs.registry, 'quay.io') }}
      ...
  ```

  Remove the `QUAY_PASSWORD` secret declaration from `workflow_call.secrets`.

  Remove or update the `registry` input description to remove mention of quay.io.

- [ ] **Commit**

  ```bash
  git add .github/workflows/webhook_image_publish.yaml \
          .github/workflows/podvm_publish.yaml
  git commit -m "ci: remove quay.io push-to-quay jobs from webhook and podvm workflows"
  ```

**Acceptance criteria:**
- No `quay.io` references remain in either file.
- No `QUAY_PASSWORD` or `QUAY_USERNAME` references remain in either file.

**Verify:** `grep -n "quay\|QUAY" .github/workflows/webhook_image_publish.yaml .github/workflows/podvm_publish.yaml` — expected: no output.

---

### Task 5: Update podvm_byom_binaries_publish workflow

**Files:**
- Modify: `.github/workflows/podvm_byom_binaries_publish.yaml`

**Steps:**

- [ ] **Replace quay login with ghcr login**

  Change:
  ```yaml
      - name: Login to Container Registry
        uses: docker/login-action@...
        with:
          registry: quay.io
          username: ${{ vars.QUAY_USERNAME }}
          password: ${{ secrets.QUAY_PASSWORD }}
  ```
  To:
  ```yaml
      - name: Login to Container Registry
        uses: docker/login-action@...
        with:
          registry: ghcr.io
          username: ${{ github.repository_owner }}
          password: ${{ secrets.GITHUB_TOKEN }}
  ```

  Add `packages: write` permission to the job's `permissions` block.

- [ ] **Change all three `REGISTRY` env defaults**

  Change every occurrence of:
  ```yaml
  REGISTRY: ${{ inputs.registry != '' && inputs.registry || 'quay.io/confidential-containers' }}
  ```
  To:
  ```yaml
  REGISTRY: ${{ inputs.registry != '' && inputs.registry || format('ghcr.io/{0}', github.repository_owner) }}
  ```

- [ ] **Remove the `QUAY_PASSWORD` secret declaration**

- [ ] **Update the `registry` input description** to remove `quay.io` example.

- [ ] **Commit**

  ```bash
  git add .github/workflows/podvm_byom_binaries_publish.yaml
  git commit -m "ci: migrate podvm BYOM binaries workflow from quay.io to ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io` references remain in the file.
- No `QUAY_PASSWORD` or `QUAY_USERNAME` references remain.

**Verify:** `grep -n "quay\|QUAY" .github/workflows/podvm_byom_binaries_publish.yaml` — expected: no output.

---

### Task 6: Update publish_images_on_push and release workflows

**Files:**
- Modify: `.github/workflows/publish_images_on_push.yaml`
- Modify: `.github/workflows/release.yaml`

**Steps:**

- [ ] **`publish_images_on_push.yaml`: simplify registry expressions**

  Change all three occurrences of:
  ```yaml
  registry: ${{ inputs.registry != '' && inputs.registry || (github.repository == 'confidential-containers/cloud-api-adaptor' && 'quay.io/confidential-containers' || format('ghcr.io/{0}', github.repository_owner)) }}
  ```
  To:
  ```yaml
  registry: ${{ inputs.registry != '' && inputs.registry || format('ghcr.io/{0}', github.repository_owner) }}
  ```

  Remove all three `QUAY_PASSWORD` secret pass-throughs.

  Update the `registry` input description to remove the `quay.io` example.

- [ ] **`release.yaml`: fix hardcoded quay registry in webhook job**

  In the `webhook` job, change:
  ```yaml
      registry: quay.io/confidential-containers
  ```
  To:
  ```yaml
      registry: ghcr.io/confidential-containers
  ```

  Remove all `QUAY_PASSWORD` secret pass-throughs from all jobs.

- [ ] **Commit**

  ```bash
  git add .github/workflows/publish_images_on_push.yaml \
          .github/workflows/release.yaml
  git commit -m "ci: migrate publish and release workflows from quay.io to ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io` references remain in either file.
- No `QUAY_PASSWORD` or `QUAY_USERNAME` references remain in either file.

**Verify:** `grep -n "quay\|QUAY" .github/workflows/publish_images_on_push.yaml .github/workflows/release.yaml` — expected: no output.

---

### Task 7: Update test-images workflow

**Files:**
- Modify: `.github/workflows/test-images.yaml`

**Steps:**

- [ ] **Replace quay login with ghcr login**

  Change:
  ```yaml
      - name: Login to Quay container Registry
        uses: docker/login-action@...
        with:
          registry: quay.io
          username: ${{ vars.QUAY_USERNAME }}
          password: ${{ secrets.QUAY_PASSWORD }}
  ```
  To:
  ```yaml
      - name: Login to ghcr.io container Registry
        uses: docker/login-action@...
        with:
          registry: ghcr.io
          username: ${{ github.repository_owner }}
          password: ${{ secrets.GITHUB_TOKEN }}
  ```

- [ ] **Update the build-and-push tags**

  Change:
  ```yaml
        tags: |
          quay.io/confidential-containers/test-images:${{env.DOCKER_TAG}}
  ```
  To:
  ```yaml
        tags: |
          ghcr.io/confidential-containers/test-images:${{env.DOCKER_TAG}}
  ```

- [ ] **Add `packages: write` permission to the `build` job** (already has `contents: read`).

- [ ] **Commit**

  ```bash
  git add .github/workflows/test-images.yaml
  git commit -m "ci: migrate test-images workflow from quay.io to ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io` references remain in the file.
- No `QUAY_PASSWORD` or `QUAY_USERNAME` references remain.

**Verify:** `grep -n "quay\|QUAY" .github/workflows/test-images.yaml` — expected: no output.

---

### Task 8: Update manual-test-helm and e2e_byom workflow defaults

**Files:**
- Modify: `.github/workflows/manual-test-helm.yaml`
- Modify: `.github/workflows/e2e_byom.yaml`

**Steps:**

- [ ] **`manual-test-helm.yaml`: update hardcoded image defaults**

  Change:
  ```yaml
        default: 'quay.io/confidential-containers/cloud-api-adaptor:latest'
  ```
  To:
  ```yaml
        default: 'ghcr.io/confidential-containers/cloud-api-adaptor:latest'
  ```

  Change:
  ```yaml
        default: 'quay.io/confidential-containers/podvm-generic-ubuntu-amd64:latest'
  ```
  To:
  ```yaml
        default: 'ghcr.io/confidential-containers/podvm-generic-ubuntu-amd64:latest'
  ```

- [ ] **`e2e_byom.yaml`: update BYOM_PODVM_IMAGE ref**

  Change:
  ```yaml
          echo "BYOM_PODVM_IMAGE=quay.io/confidential-containers/podvm-byom-e2e-image-amd64:${tag}" >> "$GITHUB_ENV"
  ```
  To:
  ```yaml
          echo "BYOM_PODVM_IMAGE=ghcr.io/confidential-containers/podvm-byom-e2e-image-amd64:${tag}" >> "$GITHUB_ENV"
  ```

- [ ] **Commit**

  ```bash
  git add .github/workflows/manual-test-helm.yaml \
          .github/workflows/e2e_byom.yaml
  git commit -m "ci: update manual-test-helm and e2e_byom workflows to use ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io` references remain in either file.

**Verify:** `grep -n "quay\|QUAY" .github/workflows/manual-test-helm.yaml .github/workflows/e2e_byom.yaml` — expected: no output.

---

### Task 9: Remove orphaned QUAY_PASSWORD refs from e2e orchestration workflows

**Files:**
- Modify: `.github/workflows/e2e_run_all.yaml`
- Modify: `.github/workflows/daily-e2e-tests.yaml`
- Modify: `.github/workflows/e2e_on_pull.yaml`

**Steps:**

- [ ] **Remove all `QUAY_PASSWORD` secret declarations and pass-throughs from each file**

  In each file, find and remove lines of the form:
  ```yaml
        QUAY_PASSWORD: ${{ secrets.QUAY_PASSWORD }}
  ```
  and any `QUAY_PASSWORD:` entries in `workflow_call.secrets` blocks.

- [ ] **Commit**

  ```bash
  git add .github/workflows/e2e_run_all.yaml \
          .github/workflows/daily-e2e-tests.yaml \
          .github/workflows/e2e_on_pull.yaml
  git commit -m "ci: remove orphaned QUAY_PASSWORD secret references from e2e workflows"
  ```

**Acceptance criteria:**
- No `QUAY_PASSWORD` or `QUAY_USERNAME` or `quay.io` references remain in any of the three files.

**Verify:** `grep -n "quay\|QUAY" .github/workflows/e2e_run_all.yaml .github/workflows/daily-e2e-tests.yaml .github/workflows/e2e_on_pull.yaml` — expected: no output.

---

### Task 10: Update Makefiles and build script defaults

**Files:**
- Modify: `src/cloud-api-adaptor/hack/build.sh`
- Modify: `src/cloud-api-adaptor/podvm/Makefile`
- Modify: `src/peerpod-ctrl/Makefile`
- Modify: `src/webhook/Makefile`

**Steps:**

- [ ] **`src/cloud-api-adaptor/hack/build.sh`**

  Change:
  ```bash
  registry="${registry:-quay.io/confidential-containers}"
  ```
  To:
  ```bash
  registry="${registry:-ghcr.io/confidential-containers}"
  ```

- [ ] **`src/cloud-api-adaptor/podvm/Makefile`**

  Change:
  ```makefile
  REGISTRY ?= quay.io/confidential-containers
  ```
  To:
  ```makefile
  REGISTRY ?= ghcr.io/confidential-containers
  ```

- [ ] **`src/peerpod-ctrl/Makefile`**

  Change:
  ```makefile
  IMG ?= quay.io/confidential-containers/peerpod-ctrl:latest
  ```
  To:
  ```makefile
  IMG ?= ghcr.io/confidential-containers/peerpod-ctrl:latest
  ```

- [ ] **`src/webhook/Makefile`**

  Change:
  ```makefile
  IMG ?= quay.io/confidential-containers/peer-pods-webhook:latest
  ```
  To:
  ```makefile
  IMG ?= ghcr.io/confidential-containers/peer-pods-webhook:latest
  ```

- [ ] **Commit**

  ```bash
  git add src/cloud-api-adaptor/hack/build.sh \
          src/cloud-api-adaptor/podvm/Makefile \
          src/peerpod-ctrl/Makefile \
          src/webhook/Makefile
  git commit -m "build: change default registry from quay.io to ghcr.io in Makefiles and build script"
  ```

**Acceptance criteria:**
- No `quay.io` default values remain in any of the four files.

**Verify:** `grep -n "quay.io" src/cloud-api-adaptor/hack/build.sh src/cloud-api-adaptor/podvm/Makefile src/peerpod-ctrl/Makefile src/webhook/Makefile` — expected: no output.

---

### Task 11: Update Dockerfile ARG defaults

**Files:**
- Modify: `src/cloud-api-adaptor/podvm/Dockerfile.podvm_byom_binaries`

**Steps:**

- [ ] **Change `BINARIES_IMG` ARG default**

  Change:
  ```dockerfile
  ARG BINARIES_IMG="quay.io/confidential-containers/podvm-binaries-ubuntu-amd64"
  ```
  To:
  ```dockerfile
  ARG BINARIES_IMG="ghcr.io/confidential-containers/podvm-binaries-ubuntu-amd64"
  ```

- [ ] **Commit**

  ```bash
  git add src/cloud-api-adaptor/podvm/Dockerfile.podvm_byom_binaries
  git commit -m "build: update podvm BYOM binaries Dockerfile default image registry to ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io/confidential-containers` ARG default remains in the file.

**Verify:** `grep "quay.io" src/cloud-api-adaptor/podvm/Dockerfile.podvm_byom_binaries` — expected: no output.

---

### Task 12: Update Helm chart values

**Files:**
- Modify: `src/cloud-api-adaptor/install/charts/peerpods/values.yaml`
- Modify: `src/cloud-api-adaptor/install/charts/peerpods/templates/daemonset.yaml`
- Modify: `src/peerpod-ctrl/chart/values.yaml`
- Modify: `src/webhook/chart/values.yaml`

**Steps:**

- [ ] **`src/cloud-api-adaptor/install/charts/peerpods/values.yaml`**

  Change:
  ```yaml
    name: quay.io/confidential-containers/cloud-api-adaptor
  ```
  To:
  ```yaml
    name: ghcr.io/confidential-containers/cloud-api-adaptor
  ```

- [ ] **`src/cloud-api-adaptor/install/charts/peerpods/templates/daemonset.yaml`**

  Change:
  ```yaml
        image: {{ .Values.image.name | default "quay.io/confidential-containers/cloud-api-adaptor" }}:{{ .Values.image.tag | default "latest" }}
  ```
  To:
  ```yaml
        image: {{ .Values.image.name | default "ghcr.io/confidential-containers/cloud-api-adaptor" }}:{{ .Values.image.tag | default "latest" }}
  ```

- [ ] **`src/peerpod-ctrl/chart/values.yaml`**

  Change:
  ```yaml
    repository: quay.io/confidential-containers/peerpod-ctrl
  ```
  To:
  ```yaml
    repository: ghcr.io/confidential-containers/peerpod-ctrl
  ```

- [ ] **`src/webhook/chart/values.yaml`**

  Change:
  ```yaml
    repository: quay.io/confidential-containers/peer-pods-webhook
  ```
  To:
  ```yaml
    repository: ghcr.io/confidential-containers/peer-pods-webhook
  ```

- [ ] **Commit**

  ```bash
  git add src/cloud-api-adaptor/install/charts/peerpods/values.yaml \
          src/cloud-api-adaptor/install/charts/peerpods/templates/daemonset.yaml \
          src/peerpod-ctrl/chart/values.yaml \
          src/webhook/chart/values.yaml
  git commit -m "helm: update default image registries from quay.io to ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io` references remain in any of the four files.

**Verify:** `grep -n "quay.io" src/cloud-api-adaptor/install/charts/peerpods/values.yaml src/cloud-api-adaptor/install/charts/peerpods/templates/daemonset.yaml src/peerpod-ctrl/chart/values.yaml src/webhook/chart/values.yaml` — expected: no output.

---

### Task 13: Update e2e test code image references

**Files:**
- Modify: `src/cloud-api-adaptor/test/e2e/versions.yaml`
- Modify: `src/cloud-api-adaptor/test/e2e/common_suite.go`

**Steps:**

- [ ] **`versions.yaml`: update test-images registry**

  Change:
  ```yaml
    curl:
      registry: "quay.io/confidential-containers/test-images"
  ```
  To:
  ```yaml
    curl:
      registry: "ghcr.io/confidential-containers/test-images"
  ```

  Leave `quay.io/confidential-containers/test/nginx`, `quay.io/nginx/nginx-unprivileged`, and `quay.io/prometheus/busybox` unchanged (third-party or un-migrated images).

- [ ] **`common_suite.go`: update hardcoded test-images refs**

  Change all five occurrences of `quay.io/confidential-containers/test-images:` to `ghcr.io/confidential-containers/test-images:`:
  - Line ~155: `image := "quay.io/prometheus/busybox:latest"` — **leave unchanged** (third-party)
  - Line ~163: `// imageName := "quay.io/confidential-containers/test-images:testuser"` — update the commented-out line too for consistency
  - Line ~183: `imageName := "quay.io/confidential-containers/test-images:testworkdir"`
  - Line ~191: `imageName := "quay.io/confidential-containers/test-images:testenv"` (two occurrences)
  - Line ~215: `imageName := "quay.io/confidential-containers/test-images:largeimage"`

- [ ] **Compile check**

  ```bash
  cd src/cloud-api-adaptor && go build ./test/e2e/...
  ```
  Expected: compiles with no errors.

- [ ] **Commit**

  ```bash
  git add src/cloud-api-adaptor/test/e2e/versions.yaml \
          src/cloud-api-adaptor/test/e2e/common_suite.go
  git commit -m "test: update e2e test image refs from quay.io to ghcr.io"
  ```

**Acceptance criteria:**
- All `quay.io/confidential-containers/test-images` refs are updated to `ghcr.io/confidential-containers/test-images`.
- Third-party registries (`quay.io/prometheus`, `quay.io/nginx`, `quay.io/curl`) are untouched.

**Verify:** `grep -n "quay.io/confidential-containers/test-images" src/cloud-api-adaptor/test/e2e/versions.yaml src/cloud-api-adaptor/test/e2e/common_suite.go` — expected: no output.

---

### Task 14: Update documentation and scripts

**Files:**
- Modify: `hack/release-helper.sh`
- Modify: `src/cloud-api-adaptor/docs/consuming-prebuilt-podvm-images.md`
- Modify: `src/cloud-api-adaptor/docs/addnewprovider.md`
- Modify: `src/cloud-api-adaptor/ibmcloud/IMPORT_PODVM_TO_VPC.md`
- Modify: `src/webhook/docs/INSTALL.md`
- Modify: `src/webhook/hack/webhook-deploy.yaml`

**Steps:**

- [ ] **`hack/release-helper.sh`**: update two text references

  Change:
  ```bash
          of the quay.io/confidential-containers/cloud-api-adaptor image
  ```
  To:
  ```bash
          of the ghcr.io/confidential-containers/cloud-api-adaptor image
  ```

  Change:
  ```bash
          quay.io/confidential-containers/cloud-api-adaptor'
  ```
  To:
  ```bash
          ghcr.io/confidential-containers/cloud-api-adaptor'
  ```

- [ ] **`consuming-prebuilt-podvm-images.md`**: update all `quay.io/confidential-containers` image URLs to `ghcr.io/confidential-containers`

- [ ] **`addnewprovider.md`**: update docker tag example

  Change:
  ```
  -t quay.io/confidential-containers/libvirt \
  ```
  To:
  ```
  -t ghcr.io/confidential-containers/libvirt \
  ```

- [ ] **`IMPORT_PODVM_TO_VPC.md`**: update `oras pull` example and surrounding text

  Change every `quay.io/confidential-containers/podvm-generic-ubuntu-amd64` to `ghcr.io/confidential-containers/podvm-generic-ubuntu-amd64`, and the general text reference to quay.

- [ ] **`src/webhook/docs/INSTALL.md`**: update make example

  Change:
  ```
  make kind-deploy IMG=quay.io/confidential-containers/peer-pods-webhook
  ```
  To:
  ```
  make kind-deploy IMG=ghcr.io/confidential-containers/peer-pods-webhook
  ```

- [ ] **`src/webhook/hack/webhook-deploy.yaml`**: update image ref

  Change:
  ```yaml
        image: quay.io/confidential-containers/peer-pods-webhook:latest
  ```
  To:
  ```yaml
        image: ghcr.io/confidential-containers/peer-pods-webhook:latest
  ```

- [ ] **Commit**

  ```bash
  git add hack/release-helper.sh \
          src/cloud-api-adaptor/docs/consuming-prebuilt-podvm-images.md \
          src/cloud-api-adaptor/docs/addnewprovider.md \
          src/cloud-api-adaptor/ibmcloud/IMPORT_PODVM_TO_VPC.md \
          src/webhook/docs/INSTALL.md \
          src/webhook/hack/webhook-deploy.yaml
  git commit -m "docs: update quay.io image references to ghcr.io"
  ```

**Acceptance criteria:**
- No `quay.io/confidential-containers` references remain in any of the six files.

**Verify:** `grep -rn "quay.io/confidential-containers" hack/release-helper.sh src/cloud-api-adaptor/docs/ src/cloud-api-adaptor/ibmcloud/ src/webhook/docs/ src/webhook/hack/` — expected: no output.

---

### Task 15: Final sweep and validation

**Steps:**

- [ ] **Run a project-wide grep for any remaining `quay.io/confidential-containers` push references**

  ```bash
  grep -rn "quay.io/confidential-containers" \
    .github/workflows/ hack/ \
    src/cloud-api-adaptor/hack/ src/cloud-api-adaptor/podvm/ \
    src/peerpod-ctrl/ src/webhook/ \
    src/cloud-api-adaptor/install/ src/cloud-api-adaptor/test/ \
    src/cloud-api-adaptor/docs/ src/cloud-api-adaptor/ibmcloud/
  ```
  Expected: no output (or only `Dockerfile.podvm_byom_docker_provider` which is explicitly out of scope).

- [ ] **Confirm third-party images are untouched**

  ```bash
  grep -rn "quay.io/prometheus\|quay.io/nginx\|quay.io/curl" \
    src/cloud-api-adaptor/test/e2e/
  ```
  Expected: still present (these are not our images).

- [ ] **Go build check across all packages**

  ```bash
  cd src/cloud-api-adaptor && go build ./...
  cd src/peerpod-ctrl && go build ./...
  cd src/webhook && go build ./...
  ```
  Expected: all compile with no errors.

- [ ] **Commit**

  ```bash
  git commit --allow-empty -m "ci: complete migration from quay.io to ghcr.io"
  ```
  (Only needed if there were any stray fixups; otherwise skip.)

**Acceptance criteria:**
- `grep -rn "quay.io/confidential-containers" .github/ hack/ src/` returns only the explicitly out-of-scope `Dockerfile.podvm_byom_docker_provider`.
- All Go packages compile.

**Verify:**
```bash
grep -rn "quay.io/confidential-containers" .github/ hack/ src/ \
  | grep -v "Dockerfile.podvm_byom_docker_provider"
```
Expected: no output.
