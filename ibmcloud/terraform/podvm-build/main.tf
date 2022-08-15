#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  worker_name = var.cluster_name != null ? "${var.cluster_name}-worker" : null
  worker_floating_ip_name = local.worker_name != null ? "${local.worker_name}-ip" : null
  worker_ip = local.worker_name != null ? data.ibm_is_instance.worker[0].primary_network_interface[0].primary_ipv4_address : var.worker_ip
  bastion_ip = local.worker_floating_ip_name != null ? data.ibm_is_floating_ip.worker[0].address : var.bastion_ip
  cos_service_instance_name_or_id = var.cos_service_instance_name != null ? var.cos_service_instance_name : var.cos_service_instance_id
  ibmcloud_api_endpoint = var.use_ibmcloud_test ? "https://test.cloud.ibm.com" : "https://cloud.ibm.com"
  podvm_image_name = var.podvm_image_name != null ? var.podvm_image_name : ""
  cos_bucket_region = var.cos_bucket_region != "" ? var.cos_bucket_region : var.region_name
  skip_verify_console = var.skip_verify_console ? "true" : ""

  is_policies_and_roles = flatten([
    for policy in data.ibm_iam_user_policy.user_policies.policies: [
      for resource in policy.resources: policy.roles
      if resource.service == "is" && resource.resource_group_id == "" && resource.resource_instance_id == ""
    ]
  ])
  has_console_administrator_role = anytrue([
    for role in local.is_policies_and_roles: true
    if role == "Console Administrator"
  ])
}

data "ibm_is_instance" "worker" {
  count = local.worker_name != null ? 1 : 0
  name = local.worker_name
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
EOF
}

data "ibm_is_subnet" "primary" {
  depends_on = [local_file.inventory]
  name = var.primary_subnet_name
}

resource "local_file" "group_vars" {
  filename = "${var.ansible_dir}/group_vars/all"
  content = <<EOF
---

ibmcloud_api_key: ${var.ibmcloud_api_key}
ibmcloud_api_endpoint: ${local.ibmcloud_api_endpoint}
ibmcloud_cos_service_instance: "${local.cos_service_instance_name_or_id}"
ibmcloud_cos_bucket: ${var.cos_bucket_name}
ibmcloud_cos_bucket_region: ${local.cos_bucket_region}
ibmcloud_region_name: ${var.region_name}
ibmcloud_vpc_name: ${var.vpc_name}
ibmcloud_vpc_subnet_name: ${var.primary_subnet_name}
ibmcloud_vpc_zone: ${data.ibm_is_subnet.primary.zone}
ibmcloud_vpc_image_name: ${local.podvm_image_name}
ibmcloud_skip_verify_console: ${local.skip_verify_console}
EOF
}

data "ibm_iam_user_policy" "user_policies" {
  ibm_id = var.ibmcloud_user_id
}

resource "ibm_iam_user_policy" "is_console_administrator_policy" {
  count = local.has_console_administrator_role ? 0 : 1
  ibm_id = var.ibmcloud_user_id
  roles = ["Console Administrator"]

  resources {
    service = "is"
  }
}

resource "null_resource" "ansible" {
  triggers = {
    inventory = resource.local_file.inventory.content
    group_vars = resource.local_file.group_vars.content
  }

  depends_on = [ibm_iam_user_policy.is_console_administrator_policy]

  provisioner "local-exec" {
    working_dir = "${var.ansible_dir}"
    command = "ansible-playbook -i inventory -u root ./playbook.yml"
  }
}

data "ibm_is_image" "podvm_image" {
  depends_on = [null_resource.ansible]
  name = local.podvm_image_name
}