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

    oras pull "ghcr.io/confidential-containers/staged-images/kbs-client:sample_only-${kbs_sha}-${arch}"
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
private_key = "/etc/https/key.pem" \
certificate = "/etc/https/cert.pem"' "$TOML_FILE"

# Append section for LocalJson reference values
cat <<EOF >> "$TOML_FILE"
[attestation_service.rvps_config.storage]
type = "LocalJson"
file_path = "/etc/rvps/reference-values.json"
EOF

cat "$TOML_FILE"

git status
install_kbs_client "${KBS_SHA}"
pushd kbs/config/kubernetes/base/

touch https-key.pem
touch https-cert.pem

# Create empty reference values file
echo -n "[]" > reference-values.json

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
        - name: https-cert
          mountPath: /etc/https/cert.pem
          subPath: https-cert.pem
        - name: https-key
          mountPath: /etc/https/key.pem
          subPath: https-key.pem
        - name: reference-values
          mountPath: /etc/rvps
          readOnly: true
      volumes:
      - name: https-cert
        secret:
          secretName: https-cert
      - name: https-key
        secret:
          secretName: https-key
      - name: reference-values
        configMap:
          name: reference-values
          items:
          - key: reference-values.json
            path: reference-values.json
EOF
ls
# Trustee only updates their staging image reliably with sha tags,
# so switch to use that and convert the version to the sha
kustomize edit add patch --path kbs-patch.yaml

kustomize edit add secret    https-key        --from-file=https-key.pem
kustomize edit add secret    https-cert       --from-file=https-cert.pem
kustomize edit add configmap reference-values --from-file=reference-values.json

kustomize edit set image kbs-container-image=ghcr.io/confidential-containers/staged-images/kbs:"${KBS_SHA}"
# For debugging
echo "Trustee deployment: $(cat kustomization.yaml). Images: $(grep -A 5 images: kustomization.yaml)"
