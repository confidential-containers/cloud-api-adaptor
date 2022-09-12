#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  template_name = "${var.cluster_name}-k8s-node"
  controlplane_name = "${var.cluster_name}-cp"
  controlplane_floating_ip_name = "${local.controlplane_name}-ip"
  worker_name = "${var.cluster_name}-worker"
  worker_floating_ip_name = "${local.worker_name}-ip"
  controlplane_ip = resource.ibm_is_instance.controlplane.primary_network_interface[0].primary_ipv4_address
  worker_ip = resource.ibm_is_instance.worker.primary_network_interface[0].primary_ipv4_address
  bastion_ip = resource.ibm_is_floating_ip.worker.address
  vpc_id = var.vpc_name != null ? data.ibm_is_vpc.vpc[0].id : var.vpc_id
  primary_security_group_id = var.primary_security_group_name != null ? data.ibm_is_security_group.primary[0].id : var.primary_security_group_id
  primary_subnet_id = var.primary_subnet_name != null ? data.ibm_is_subnet.primary[0].id : var.primary_subnet_id
}

resource "ibm_is_ssh_key" "created_ssh_key" {
  # Create the ssh key only if the public key is set
  count = var.ssh_pub_key == "" ? 0 : 1
  name = var.ssh_key_name
  public_key = var.ssh_pub_key
}

data "ibm_is_ssh_key" "ssh_key" {
  # Wait if the key needs creating first
  depends_on = [ibm_is_ssh_key.created_ssh_key]
  name = var.ssh_key_name
}

data "ibm_is_image" "k8s_node" {
  name = var.image_name
}

data "ibm_is_vpc" "vpc" {
  count = var.vpc_name != null ? 1 : 0
  name = var.vpc_name
}

data "ibm_is_subnet" "primary" {
  count = var.primary_subnet_name != null ? 1 : 0
  name = var.primary_subnet_name
}

data "ibm_is_security_group" "primary" {
  count = var.primary_security_group_name != null ? 1 : 0
  name = var.primary_security_group_name
}

resource "ibm_is_instance_template" "k8s_node" {
  name    = local.template_name
  image   = data.ibm_is_image.k8s_node.id
  profile = var.instance_profile_name
  vpc     = local.vpc_id
  zone    = var.zone_name
  keys    = [data.ibm_is_ssh_key.ssh_key.id]

  primary_network_interface {
    subnet = local.primary_subnet_id
    security_groups = [local.primary_security_group_id]
  }
}

resource "ibm_is_instance" "controlplane" {
  name              = local.controlplane_name
  instance_template = ibm_is_instance_template.k8s_node.id
}

resource "ibm_is_floating_ip" "controlplane" {
    name = local.controlplane_floating_ip_name
    target = ibm_is_instance.controlplane.primary_network_interface[0].id
}

resource "ibm_is_instance" "worker" {
  name              = local.worker_name
  instance_template = ibm_is_instance_template.k8s_node.id
}


resource "ibm_is_floating_ip" "worker" {
  name = local.worker_floating_ip_name
  target = ibm_is_instance.worker.primary_network_interface[0].id
}

resource "local_file" "inventory" {
  filename = "${var.ansible_dir}/inventory"
  content = <<EOF
[cluster]
${local.controlplane_ip}
${local.worker_ip}

[cluster:vars]
ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ProxyCommand="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -W %h:%p root@${local.bastion_ip}"'
EOF
}

resource "null_resource" "ansible" {
  triggers = {
    inventry = resource.local_file.inventory.content
    group_vars = resource.local_file.group_vars.content
  }

  provisioner "local-exec" {
    working_dir = "${var.scripts_dir}"
    command = "./keygen.sh --bastion ${local.bastion_ip} ${local.controlplane_ip} ${local.worker_ip}"
  }

  provisioner "local-exec" {
    working_dir = "${var.ansible_dir}"
    command = "ansible-playbook -i inventory -u root ./kube-playbook.yml && ansible-playbook -i inventory -u root ./kata-playbook.yml"
  }
  
  provisioner "local-exec" {
    working_dir = "${var.scripts_dir}"
    command = "./setup.sh --bastion ${local.bastion_ip} --control-plane ${local.controlplane_ip} --workers ${local.worker_ip}"
  }
}

data "ibm_is_instance" "provisioned_worker" {
  depends_on = [null_resource.ansible]
  name = local.worker_name
}

resource "local_file" "group_vars" {
  filename = "${var.ansible_dir}/group_vars/all"
  content = <<EOF
---

cloud_api_adaptor_repo: ${var.cloud_api_adaptor_repo}
cloud_api_adaptor_branch: ${var.cloud_api_adaptor_branch}
kata_containers_repo: ${var.kata_containers_repo}
kata_containers_branch: ${var.kata_containers_branch}
containerd_repo: ${var.containerd_repo}
containerd_branch: ${var.containerd_branch}
EOF
}
