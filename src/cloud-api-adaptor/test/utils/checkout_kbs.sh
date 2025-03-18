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

TOML_FILE="kbs/config/kubernetes/base/kbs-config.toml"

sed -i '/insecure_http = true/c\
insecure_api = true \
private_key = "/etc/kbs2/https-key.pem" \
certificate = "/etc/kbs1/https-cert.pem"' "$TOML_FILE"

cat "$TOML_FILE"

git status
install_kbs_client "${KBS_SHA}"
pushd kbs/config/kubernetes/base/

touch https-key.pem
touch https-cert.pem

cat <<EOF > kbs-patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kbs
spec:
  template:
    spec:
      containers:
      - name: kbs
        volumeMounts:
        - name: kbs-https-certificate
          mountPath: /etc/kbs1
        - name: kbs-https-key
          mountPath: /etc/kbs2
      volumes:
      - name: kbs-https-certificate
        secret:
          secretName: kbs-https-certificate
      - name: kbs-https-key
        secret:
          secretName: kbs-https-key
EOF
ls
# Trustee only updates their staging image reliably with sha tags,
# so switch to use that and convert the version to the sha
kustomize edit add patch --path kbs-patch.yaml
kustomize edit add secret kbs-https-key --from-file=https-key.pem
kustomize edit add secret kbs-https-certificate --from-file=https-cert.pem
kustomize edit set image kbs-container-image=ghcr.io/confidential-containers/staged-images/kbs:"${KBS_SHA}"
# For debugging
echo "Trustee deployment: $(cat kustomization.yaml). Images: $(grep -A 5 images: kustomization.yaml)"
