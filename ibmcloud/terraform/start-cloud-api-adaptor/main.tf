#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  worker_name = "${var.cluster_name}-worker"
  worker_floating_ip_name = "${local.worker_name}-ip"
  podvm_image_id = data.ibm_is_image.podvm_image.id
  worker_ip = data.ibm_is_instance.worker.primary_network_interface[0].primary_ipv4_address
  bastion_ip = data.ibm_is_floating_ip.worker.address
  zone_name = data.ibm_is_subnet.primary_subnet.zone
  vpc_id = data.ibm_is_vpc.vpc.id
  ssh_key_id = data.ibm_is_ssh_key.ssh_key.id
  primary_subnet_id = data.ibm_is_subnet.primary_subnet.id
  primary_security_group_id = data.ibm_is_security_group.primary_security_group.id
  ibmcloud_api_endpoint = var.use_ibmcloud_test ? "https://test.cloud.ibm.com" : "https://cloud.ibm.com"
}

data "ibm_is_instance" "worker" {
  name = local.worker_name
}

data "ibm_is_image" "podvm_image" {
  name = var.podvm_image_name
}

data "ibm_is_vpc" "vpc" {
  name = var.vpc_name
}

data "ibm_is_ssh_key" "ssh_key" {
  name = var.ssh_key_name
}

data "ibm_is_subnet" "primary_subnet" {
  name = var.primary_subnet_name
}

data "ibm_is_security_group" "primary_security_group" {
  name = var.primary_security_group_name
}

data "ibm_is_floating_ip" "worker" {
  name = local.worker_floating_ip_name
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

resource "local_file" "group_vars" {
  filename = "./ansible/group_vars/all"
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
EOF
}

resource "null_resource" "ansible" {
  triggers = {
    inventory = resource.local_file.inventory.content
    group_vars = resource.local_file.group_vars.content
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