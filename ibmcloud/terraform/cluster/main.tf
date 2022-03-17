#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
  template_name = "${var.cluster_name}-k8s-node"
  controlplane_name = "${var.cluster_name}-cp"
  worker_name = "${var.cluster_name}-worker"
  worker_floating_ip_name = "${local.worker_name}-ip"
  controlplane_ip = resource.ibm_is_instance.controlplane.primary_network_interface[0].primary_ipv4_address
  worker_ip = resource.ibm_is_instance.worker.primary_network_interface[0].primary_ipv4_address
  bastion_ip = resource.ibm_is_floating_ip.worker.address
}

data "ibm_is_ssh_key" "ssh_key" {
  name = var.ssh_key_name
}

data "ibm_is_image" "k8s_node" {
    name = var.image_name
}

data "ibm_is_vpc" "vpc" {
  name = var.vpc_name
}

data "ibm_is_subnet" "primary" {
  name = var.primary_subnet_name
}

data "ibm_is_security_group" "primary" {
  name = var.primary_security_group_name
}

resource "ibm_is_instance_template" "k8s_node" {
  name    = local.template_name
  image   = data.ibm_is_image.k8s_node.id
  profile = var.instance_profile_name
  vpc     = data.ibm_is_vpc.vpc.id
  zone    = var.zone_name
  keys    = [data.ibm_is_ssh_key.ssh_key.id]

  primary_network_interface {
    subnet = data.ibm_is_subnet.primary.id
    security_groups = [data.ibm_is_security_group.primary.id]
  }
}

resource "ibm_is_instance" "controlplane" {
  name              = local.controlplane_name
  instance_template = ibm_is_instance_template.k8s_node.id
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
  filename = "./ansible/inventory"
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
  }
  provisioner "local-exec" {
    command = "scripts/keygen.sh --bastion ${local.bastion_ip} ${local.controlplane_ip} ${local.worker_ip}"
  }
  provisioner "local-exec" {
    working_dir = "./ansible"
    command = "ansible-playbook -i inventory -u root ./playbook.yml"
  }
  provisioner "local-exec" {
    command = "./scripts/setup.sh --bastion ${local.bastion_ip} --control-plane ${local.controlplane_ip} --workers ${local.worker_ip}"
  }
}
