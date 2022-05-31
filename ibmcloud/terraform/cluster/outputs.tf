#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

output "cluster_name" {
  value = var.cluster_name
}

output "worker_ip" {
  value = data.ibm_is_instance.provisioned_worker.primary_network_interface[0].primary_ipv4_address
}

output "bastion_ip" {
  value = ibm_is_floating_ip.worker.address
}

output "ssh_key_id" {
  value = data.ibm_is_ssh_key.ssh_key.id
}