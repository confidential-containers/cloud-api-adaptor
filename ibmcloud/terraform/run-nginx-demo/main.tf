#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  worker_name = "${var.cluster_name}-worker"
  worker_floating_ip_name = "${local.worker_name}-ip"
  worker_ip = data.ibm_is_instance.worker.primary_network_interface[0].primary_ipv4_address
  bastion_ip = data.ibm_is_floating_ip.worker.address
}

data "ibm_is_floating_ip" "worker" {
  name = local.worker_floating_ip_name
}

data "ibm_is_instance" "worker" {
  name = local.worker_name
}

resource "local_file" "inventory" {
  filename = "./ansible/inventory"
  content = <<EOF
[cluster]
${local.worker_ip}

[cluster:vars]
ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ProxyCommand="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -W %h:%p root@${local.bastion_ip}"'
EOF
}

resource "null_resource" "ansible" {
  triggers = {
    inventory = resource.local_file.inventory.content
  }
  provisioner "local-exec" {
    working_dir = "./ansible"
    command = "ansible-playbook -i inventory -u root ./apply_playbook.yml"
  }

  provisioner "local-exec" {
    when    = destroy
    working_dir = "./ansible"
    command = "ansible-playbook -i inventory -u root ./destroy_playbook.yml"
  }
}

