#!/bin/bash
#
# Copyright (c) 2024 IBM Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

export TEST_DIR=$(cd "$(dirname "$(realpath "$0")")/../"; pwd)

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

YAML_FILE="kbs/config/kubernetes/base/deployment.yaml"

yq -i '.spec.template.spec.containers[0].volumeMounts += [{"name": "kbs-https-certificate", "mountPath": "/etc/https-cert.pem"}, {"name": "kbs-https-key", "mountPath": "/etc/https-key.pem"}]' "$YAML_FILE"

yq -i '.spec.template.spec.volumes += [{"name": "kbs-https-certificate", "secret": {"secretName": "kbs-https-certificate"}}, {"name": "kbs-https-key", "secret": {"secretName": "kbs-https-key"}}]' "$YAML_FILE"

cat "$YAML_FILE"
pwd
ls
TOML_FILE="kbs/config/kubernetes/base/kbs-config.toml"

sed -i '/insecure_http = true/c\
insecure_api = true \
private_key = "/etc/https-key.pem" \
certificate = "/etc/https-cert.pem"' "$TOML_FILE"

cat "$TOML_FILE"

KUSTOMIZATION_FILE="kbs/config/kubernetes/base/kustomization.yaml"

yq eval '
  .secretGenerator += [
    {"files": ["https-key.pem"], "name": "kbs-https-key"},
    {"files": ["https-cert.pem"], "name": "kbs-https-certificate"}
  ]
' -i "$KUSTOMIZATION_FILE"

cat "$KUSTOMIZATION_FILE"

git status
install_kbs_client "${KBS_SHA}"
pushd kbs/config/kubernetes/base/
# Trustee only updates their staging image reliably with sha tags,
# so switch to use that and convert the version to the sha
kustomize edit set image kbs-container-image=ghcr.io/confidential-containers/staged-images/kbs:"${KBS_SHA}"
# For debugging
echo "Trustee deployment: $(cat kustomization.yaml). Images: $(grep -A 5 images: kustomization.yaml)"

