#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

output "podvm_image_id" {
    value = data.ibm_is_image.podvm_image.id
}