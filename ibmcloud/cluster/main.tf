#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

data "ibm_is_vpc" "vpc" {
  count = var.vpc_name == "" ? 0 : 1
  name  = var.vpc_name
}

data "ibm_is_subnet" "subnet" {
  count = var.subnet_name == "" ? 0 : 1
  name  = var.subnet_name
}

module "vpc" {
  # Create new vpc ans subnet only if vpc_name is not set
  count        = var.vpc_name == "" ? 1 : 0
  source       = "./vpc"
  cluster_name = var.cluster_name
  zone         = var.zone
}

locals {
  vpc_id            = var.vpc_name == "" ? module.vpc[0].vpc_id : data.ibm_is_vpc.vpc[0].id
  subnet_id         = var.vpc_name == "" ? module.vpc[0].subnet_id : data.ibm_is_subnet.subnet[0].id
  security_group_id = var.vpc_name == "" ? module.vpc[0].security_group_id : data.ibm_is_vpc.vpc[0].default_security_group
}

data "ibm_resource_group" "default_group" {
  is_default = "true"
}

data "ibm_is_image" "node_image" {
  name = var.node_image
}

resource "ibm_is_ssh_key" "created_ssh_key" {
  # Create the ssh key only if the public key is set
  count      = var.ssh_pub_key == "" ? 0 : 1
  name       = var.ssh_key_name
  public_key = var.ssh_pub_key
}

data "ibm_is_ssh_key" "ssh_key" {
  # Wait if the key needs creating first
  depends_on = [ibm_is_ssh_key.created_ssh_key]
  name       = var.ssh_key_name
}


resource "ibm_is_instance_template" "node_template" {
  name    = "${var.cluster_name}-node-template"
  image   = data.ibm_is_image.node_image.id
  profile = var.node_profile
  vpc     = local.vpc_id
  zone    = var.zone
  keys    = [data.ibm_is_ssh_key.ssh_key.id]

  primary_network_interface {
    subnet          = local.subnet_id
    security_groups = [local.security_group_id]
  }
}

module "nodes" {
  source                    = "./node"
  count                     = var.nodes
  node_name                 = "${var.cluster_name}-node-${count.index}"
  node_instance_template_id = ibm_is_instance_template.node_template.id
}

resource "local_file" "inventory" {
  content = templatefile("${path.module}/ansible/inventory.tmpl",
    {
      ip_addrs = [for k, w in module.nodes : w.public_ip],
    }
  )
  filename = "${path.module}/ansible/inventory"
}


resource "null_resource" "ansible" {
  triggers = {
    inventory = resource.local_file.inventory.content
  }
  provisioner "local-exec" {
    working_dir = "./ansible"
    command     = "ansible-playbook -i inventory -u root containerd.yaml -e containerd_release_version=${var.containerd_version} -e kube_version=${var.kube_version}"
  }
}

resource "null_resource" "kubeadm" {
  depends_on = [
    null_resource.ansible
  ]
  provisioner "local-exec" {
    command = "./kube-init.sh ${module.nodes[0].private_ip}"
  }
}

resource "null_resource" "label_nodes" {
  depends_on = [
    null_resource.kubeadm
  ]
  provisioner "local-exec" {
    command = "./label-nodes.sh ${var.region} ${var.zone} ${local.subnet_id}"
  }
}
