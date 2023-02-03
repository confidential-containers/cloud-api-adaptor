#
# (C) Copyright IBM Corp. 2023.
# SPDX-License-Identifier: Apache-2.0
#

provider "ibm" {
    ibmcloud_api_key = var.ibmcloud_api_key
    region = var.region
}
