#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  worker_name = var.cluster_name != null ? "${var.cluster_name}-worker" : null
  worker_floating_ip_name = local.worker_name != null ? "${local.worker_name}-ip" : null
  worker_ip = local.worker_name != null ? data.ibm_is_instance.worker[0].primary_network_interface[0].primary_ipv4_address : var.worker_ip
  bastion_ip = local.worker_floating_ip_name != null ? data.ibm_is_floating_ip.worker[0].address : var.bastion_ip
  vpc_id = var.vpc_name != null ? data.ibm_is_vpc.vpc[0].id : var.vpc_id
  podvm_image_id = var.podvm_image_name != null ? data.ibm_is_image.podvm_image[0].id : var.podvm_image_id
}

data "ibm_is_floating_ip" "worker" {
  count = local.worker_floating_ip_name != null ? 1 : 0
  name = local.worker_floating_ip_name
}

data "ibm_is_instance" "worker" {
  count = local.worker_name != null ? 1 : 0
  name = local.worker_name
}

data "ibm_is_vpc" "vpc" {
  count = var.vpc_name != null ? 1 : 0
  name = var.vpc_name
}

data "ibm_is_image" "podvm_image" {
  count = var.podvm_image_name != null ? 1 : 0
  name = var.podvm_image_name
}

resource "local_file" "inventory" {
  filename = "${var.ansible_dir}/inventory"
  content = <<EOF
[cluster]
${local.worker_ip}

[cluster:vars]
ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ProxyCommand="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -W %h:%p root@${local.bastion_ip}"'

# Peer pod VM image ID: ${local.podvm_image_id}
# 
# Add the inbound, outbound and SSH security group rules for Ansible playbooks that have destroy playbooks so that the destroy playbook is executed before the
# security group rules are deleted
# SSH security group rule ID: ${var.ssh_security_group_rule_id}
# Inbound security group rule ID: ${var.inbound_security_group_rule_id}
# Outbound security group rule ID: ${var.outbound_security_group_rule_id}
EOF
}

resource "null_resource" "ansible" {
  triggers = {
    inventory = resource.local_file.inventory.content
  }

  provisioner "local-exec" {
    working_dir = "${var.ansible_dir}"
    command = "ansible-playbook -i inventory -u root ./apply_playbook.yml"
  }

  # working_dir in 'provisioner "local-exec" needs to be a fixed value in a destroy-time provisioner. Do the following to work around needing to use a fixed working_dir

  provisioner "local-exec" {
    command = "mkdir -p ansible && cp ${var.ansible_dir}/inventory ./ansible/inventory_nginx_demo && cp ${var.ansible_dir}/destroy_playbook.yml ./ansible/destroy_nginx_demo_playbook.yml"
  }

  provisioner "local-exec" {
    when    = destroy
    working_dir = "./ansible"
    command = "ansible-playbook -i inventory_nginx_demo -u root ./destroy_nginx_demo_playbook.yml && rm -f destroy_nginx_demo_playbook.yml inventory_nginx_demo"
  }
}

data "ibm_is_instances" "instances" {
  depends_on = [null_resource.ansible]
  vpc = local.vpc_id
}

module "check_podvm_instance" {
    source = "../check-podvm-instance"
    ibmcloud_api_key = var.ibmcloud_api_key
    region_name = var.region_name
    virtual_server_instances = data.ibm_is_instances.instances
    podvm_image_id = local.podvm_image_id
}