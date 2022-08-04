#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  worker_name = var.cluster_name != null ? "${var.cluster_name}-worker" : null
  worker_floating_ip_name = local.worker_name != null ? "${local.worker_name}-ip" : null
  podvm_image_id = var.podvm_image_name != null ? data.ibm_is_image.podvm_image[0].id : var.podvm_image_id
  worker_ip = local.worker_name != null ? data.ibm_is_instance.worker[0].primary_network_interface[0].primary_ipv4_address : var.worker_ip
  bastion_ip = local.worker_floating_ip_name != null ? data.ibm_is_floating_ip.worker[0].address : var.bastion_ip
  zone_name = data.ibm_is_subnet.primary_subnet_by_id.zone
  vpc_id = var.vpc_name != null ? data.ibm_is_vpc.vpc[0].id : var.vpc_id
  ssh_key_id = var.ssh_key_name != null ? data.ibm_is_ssh_key.ssh_key[0].id : var.ssh_key_id
  primary_subnet_id = var.primary_subnet_name != null ? data.ibm_is_subnet.primary_subnet_by_name[0].id : var.primary_subnet_id
  primary_security_group_id = var.primary_security_group_name != null ? data.ibm_is_security_group.primary_security_group[0].id : var.primary_security_group_id
  resource_group_id = var.resource_group_name != null ? data.ibm_resource_group.group[0].id : data.ibm_resource_group.default_group.id
  ibmcloud_api_endpoint = var.use_ibmcloud_test ? "https://test.cloud.ibm.com" : "https://cloud.ibm.com"
}

data "ibm_resource_group" "group" {
  count = var.resource_group_name != null ? 1 : 0
  name = var.resource_group_name
}

data "ibm_resource_group" "default_group" {
  is_default = "true"
}

data "ibm_is_instance" "worker" {
  count = local.worker_name != null ? 1 : 0
  name = local.worker_name
}

data "ibm_is_image" "podvm_image" {
  count = var.podvm_image_name != null ? 1 : 0
  name = var.podvm_image_name
}

data "ibm_is_vpc" "vpc" {
  count = var.vpc_name != null ? 1 : 0
  name = var.vpc_name
}

data "ibm_is_ssh_key" "ssh_key" {
  count = var.ssh_key_name != null ? 1 : 0
  name = var.ssh_key_name
}

data "ibm_is_subnet" "primary_subnet_by_name" {
  count = var.primary_subnet_name != null ? 1 : 0
  name = var.primary_subnet_name
}

data "ibm_is_subnet" "primary_subnet_by_id" {
  identifier = local.primary_subnet_id
}

data "ibm_is_security_group" "primary_security_group" {
  count = var.primary_security_group_name != null ? 1 : 0
  name = var.primary_security_group_name
}

data "ibm_is_floating_ip" "worker" {
  count = local.worker_floating_ip_name != null ? 1 : 0
  name = local.worker_floating_ip_name
}

resource "local_file" "inventory" {
  filename = "${var.ansible_dir}/inventory"
  content = <<EOF
[cluster]
${local.worker_ip}

[cluster:vars]
ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ProxyCommand="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -W %h:%p root@${local.bastion_ip}"'
# 
# Add the inbound, outbound and SSH security group rules for Ansible playbooks that have destroy playbooks so that the destroy playbook is executed before the
# security group rules are deleted
# SSH security group rule ID: ${var.ssh_security_group_rule_id}
# Inbound security group rule ID: ${var.inbound_security_group_rule_id}
# Outbound security group ruke ID: ${var.outbound_security_group_rule_id}
EOF
}

resource "local_file" "group_vars" {
  filename = "${var.ansible_dir}/group_vars/all"
  content = <<EOF
---

ibmcloud_api_key: ${var.ibmcloud_api_key}
ibmcloud_api_endpoint: ${local.ibmcloud_api_endpoint}
ibmcloud_vpc_ssh_key_id: ${local.ssh_key_id}
ibmcloud_vpc_podvm_image_id: ${local.podvm_image_id}
ibmcloud_vpc_podvm_instance_profile_name: ${var.instance_profile_name}
ibmcloud_vpc_region_name: ${var.region_name}
ibmcloud_vpc_zone_name: ${local.zone_name}
ibmcloud_vpc_subnet_id: ${local.primary_subnet_id}
ibmcloud_vpc_security_group_id: ${local.primary_security_group_id}
ibmcloud_vpc_id: ${local.vpc_id}
ibmcloud_resource_group_id: ${local.resource_group_id}
ibmcloud_cri_runtime_endpoint: ${var.cri_runtime_endpoint}
EOF
}

resource "null_resource" "ansible" {
  triggers = {
    inventory = resource.local_file.inventory.content
    group_vars = resource.local_file.group_vars.content
  }

  provisioner "local-exec" {
    working_dir = "${var.ansible_dir}"
    command = "ansible-playbook -i inventory -u root ./apply_playbook.yml"
  }

  # working_dir in 'provisioner "local-exec" needs to be a fixed value in a destroy-time provisioner. Do the following to work around needing to use a fixed working_dir

  provisioner "local-exec" {
    command = "mkdir -p ansible && cp ${var.ansible_dir}/inventory ./ansible/inventory_cloud_api_adaptor && cp ${var.ansible_dir}/destroy_playbook.yml ./ansible/destroy_cloud_api_adaptor_playbook.yml"
  }

  provisioner "local-exec" {
    when    = destroy
    working_dir = "./ansible"
    command = "ansible-playbook -i inventory_cloud_api_adaptor -u root ./destroy_cloud_api_adaptor_playbook.yml && rm -f inventory_cloud_api_adaptor destroy_cloud_api_adaptor_playbook.yml"
  }
}

data "ibm_is_image" "cloud_api_adaptor_podvm_image" {
  depends_on = [null_resource.ansible]
  identifier = local.podvm_image_id
}