#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  worker_name = "${var.cluster_name}-worker"
  worker_floating_ip_name = "${local.worker_name}-ip"
  worker_ip = data.ibm_is_instance.worker.primary_network_interface[0].primary_ipv4_address
  bastion_ip = data.ibm_is_floating_ip.worker.address
  zone_name = data.ibm_is_subnet.primary.zone
  cos_service_instance_name = var.cos_service_instance_name != null ? var.cos_service_instance_name : "${var.cluster_name}-cos-service-instance"
  cos_bucket_name = var.cos_bucket_name != null ? var.cos_bucket_name : "${var.cluster_name}-cos-bucket"
  ibmcloud_api_endpoint = var.use_ibmcloud_test ? "https://test.cloud.ibm.com" : "https://cloud.ibm.com"
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
  name = local.worker_name
}

data "ibm_is_floating_ip" "worker" {
  name = local.worker_floating_ip_name
}

data "ibm_is_subnet" "primary" {
  name = var.primary_subnet_name
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
ibmcloud_cos_service_instance: ${local.cos_service_instance_name}
ibmcloud_cos_bucket: ${local.cos_bucket_name}
ibmcloud_region_name: ${var.region_name}
ibmcloud_vpc_name: ${var.vpc_name}
ibmcloud_vpc_subnet_name: ${var.primary_subnet_name}
ibmcloud_vpc_zone: ${local.zone_name}
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
    working_dir = "./ansible"
    command = "ansible-playbook -i inventory -u root ./playbook.yml"
  }
}