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

data "ibm_is_instance" "podvm_instance" {
  name = one(local.peer_pod_vms)
}