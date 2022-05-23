#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  peer_pod_vms = [ 
    for instance in data.ibm_is_instances.instances.instances: instance.name
    if instance.image == data.ibm_is_image.podvm_image.id
  ]
}

data "ibm_is_image" "podvm_image" {
  name = var.podvm_image_name
}

data "ibm_is_instances" "instances" {
  vpc_name = var.vpc_name
}

resource "null_resource" "check_passed" {
  count = length(local.peer_pod_vms) == 1 ? 1 : 0
  provisioner "local-exec" {
    command = "echo 1 Virtual Server instance ${local.peer_pod_vms[0]} that uses the peer pod VM found. Test passed"
  }
}

resource "null_resource" "check_failed" {
  count = length(local.peer_pod_vms) != 1 ? 1 : 0
  provisioner "local-exec" {
    command = "echo ${length(local.peer_pod_vms)} Virtual Server instances that use the peer pod VM found. Test failed\nexit 1"
  }
}

