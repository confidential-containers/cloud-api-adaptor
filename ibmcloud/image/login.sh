#!/bin/bash
#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit -o pipefail -o nounset

cd "$(dirname "${BASH_SOURCE[0]}")"

api_key=${IBMCLOUD_API_KEY-}
api_endpoint=${IBMCLOUD_API_ENDPOINT-https://cloud.ibm.com}
region=${IBMCLOUD_VPC_REGION:-jp-tok}
export IBMCLOUD_VERSION_CHECK=false

ibmcloud config --http-timeout 60
ibmcloud config --color false

if ! ibmcloud iam oauth-tokens &> /dev/null; then
    if [[ -n "$api_key" ]]; then
        opts=(--apikey "$api_key")
    else
        opts=(--sso)
    fi
    ibmcloud login -a "$api_endpoint" -r "$region" "${opts[@]}"
fi

required_plugins=(vpc-infrastructure cloud-object-storage)
installed_plugins=($(ibmcloud plugin list --output json | jq -r '.[].Name'))

for plugin in "${required_plugins[@]}"; do
    if ! [[ -n "${installed_plugins-}" && " ${installed_plugins[*]} " =~ " $plugin " ]]; then
        ibmcloud plugin install "$plugin"
    fi
done
