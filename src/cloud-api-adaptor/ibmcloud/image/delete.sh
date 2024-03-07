#!/bin/bash
#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit -o pipefail -o nounset

cd "$(dirname "${BASH_SOURCE[0]}")"

IBMCLOUD_HOME=$(pwd -P)
export IBMCLOUD_HOME
./login.sh

cos_bucket="paidvpcimagebucket"

for img in "$@"; do
    img=${img%.qcow2}
    ibmcloud is image-delete --force "$img"
    ibmcloud cos object-delete --bucket "$cos_bucket" --key "$img.qcow2" --force
done
