#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  podvm_image_id = var.podvm_image_name != null ? data.ibm_is_image.podvm_image[0].id : var.podvm_image_id
  number_of_peer_pod_vms = length(local.peer_pod_vms)
  peer_pod_vms = [ 
    for instance in var.virtual_server_instances.instances: instance.name
    if instance.image == local.podvm_image_id
  ]
}

data "ibm_is_image" "podvm_image" {
  count = var.podvm_image_name != null ? 1 : 0
  name = var.podvm_image_name
}

resource "null_resource" "check" {
  provisioner "local-exec" {
    command = "if [ ${local.number_of_peer_pod_vms} -eq 1 ]; then echo 1 Virtual Server instance ${local.peer_pod_vms[0]} that uses the peer pod VM found. Test passed; else echo ${local.number_of_peer_pod_vms} Virtual Server instances that use the peer pod VM found. Test failed; exit 1; fi"
  }
}

