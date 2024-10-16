#!/bin/bash
#
# Copyright (c) 2024 IBM Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

TEST_DIR=$(cd "$(dirname "$(realpath "$0")")/../"; pwd)

VERSIONS_YAML_PATH=$(realpath "${TEST_DIR}/../versions.yaml")

KBS_REPO=$(yq -e '.git.kbs.url' "${VERSIONS_YAML_PATH}")
KBS_VERSION=$(yq -e '.git.kbs.reference' "${VERSIONS_YAML_PATH}")

install_kbs_client() {
    kbs_sha=$1
    arch=$(uname -m)

    oras pull "ghcr.io/confidential-containers/staged-images/kbs-client:sample_only-${arch}-linux-gnu-${kbs_sha}"
    chmod +x ./kbs-client
}

rm -rf "${TEST_DIR}/trustee"
git clone "${KBS_REPO}" "${TEST_DIR}/trustee"
pushd "${TEST_DIR}/trustee"
git checkout "${KBS_VERSION}"
KBS_SHA="$(git rev-parse HEAD)"

install_kbs_client "${KBS_SHA}"

pushd kbs/config/kubernetes/base/
# Trustee only updates their staging image reliably with sha tags,
# so switch to use that and convert the version to the sha
kustomize edit set image kbs-container-image=ghcr.io/confidential-containers/staged-images/kbs:"${KBS_SHA}"
# For debugging
echo "Trustee deployment: $(cat kustomization.yaml). Images: $(grep -A 5 images: kustomization.yaml)"
