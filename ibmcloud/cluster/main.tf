#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

module "vpc" {
  source       = "./vpc"
  cluster_name = var.cluster_name
  zone         = var.zone
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
  vpc     = module.vpc.vpc_id
  zone    = var.zone
  keys    = [data.ibm_is_ssh_key.ssh_key.id]

  primary_network_interface {
    subnet          = module.vpc.subnet_id
    security_groups = [module.vpc.security_group_id]
  }
}

module "nodes" {
  source                      = "./node"
  count                       = var.nodes
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
    command     = "ansible-playbook -i inventory -u root containerd.yaml -e containerd_release_version=${var.containerd_version}"
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
