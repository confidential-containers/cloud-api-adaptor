variable "ibmcloud_api_key" {
    sensitive = true
}

variable "cluster_name" {
    description = "Prefix for the Kubernetes control plane and worker instances. Must be provided if worker_ip and bastion_ip are not provided"
    default = null
}

variable "worker_ip" {
    description = "Internal ipv4 address assigned to the worker instance. Must be provided if cluster_name is not provided"
    default = null
}

variable "bastion_ip" {
    description = "Floating ipv4 address assigned to the worker instance. Must be provided if cluster_name is not provided"
    default = null
}

variable "region_name" {
    default = "jp-tok"
}

variable "podvm_image_id" {
    description = "ID of the VPC Custom Image used for the peer pod VM. Must be provided if podvm_image_name is not provided"
    default = null
}

variable "podvm_image_name" {
    description = "Name of the VPC Custom Image used for the peer pod VM. Must be provided if podvm_image_id is not provided"
    default = null
}

variable "vpc_name" {
    description = "Name of the VPC. Must be provided if vpc_id is not provided"
    default = null
}

variable "vpc_id" {
    description = "ID of the VPC. Must be provided if vpc_name is not provided"
    default = null
}

variable "ssh_security_group_rule_id" {
    description = "Only set ssh_security_group_rule_id when this template is called as a module to enforce that it is dependent on the SSH security group rule"
    default = ""
}

variable "inbound_security_group_rule_id" {
    description = "Only set inbound_security_group_rule_id when this template is called as a module to enforce that it is dependent on the inbound security group rule"
    default = ""
}

variable "outbound_security_group_rule_id" {
    description = "Only set outbound_security_group_rule_id when this template is called as a module to enforce that it is dependent on the outbound security group rule"
    default = ""
}

variable "ansible_dir" {
    description = "Subdirectory for Ansible playbook, inventory and vars files"
    default = "./ansible"
}
