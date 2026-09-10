#!/bin/bash
#
# Copyright (c) 2024 IBM Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

TEST_DIR=$(cd "$(dirname "$(realpath "$0")")/../"; pwd)

VERSIONS_YAML_PATH=$(realpath "${TEST_DIR}/../versions.yaml")
VERSIONS_PY=$(realpath "${TEST_DIR}/../hack/versions.py")

KBS_VERSION=$("${VERSIONS_PY}" -f "${VERSIONS_YAML_PATH}" -q 'git.kbs.reference')

install_kbs_client() {
    local kbs_sha=$1
    local arch
    arch=$(uname -m)
    local tmpdir
    tmpdir=$(mktemp -d)
    (cd "${tmpdir}" && oras pull "ghcr.io/confidential-containers/staged-images/kbs-client:sample_only-${kbs_sha}-${arch}")
    chmod +x "${tmpdir}/kbs-client"
    sudo mv "${tmpdir}/kbs-client" /usr/local/bin/kbs-client
    rm -rf "${tmpdir}"
}

install_kbs_client "${KBS_VERSION}"
