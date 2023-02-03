#
# (C) Copyright IBM Corp. 2023.
# SPDX-License-Identifier: Apache-2.0
#

resource "ibm_is_instance" "node" {
  name              = var.node_name
  instance_template = var.node_instance_template_id
}

resource "ibm_is_floating_ip" "node" {
  name   = "${var.node_name}-ip"
  target = ibm_is_instance.node.primary_network_interface[0].id
}
