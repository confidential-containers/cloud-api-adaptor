#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

output "cloud_api_adaptor_podvm_image_id" {
    value = data.ibm_is_image.cloud_api_adaptor_podvm_image.id
}