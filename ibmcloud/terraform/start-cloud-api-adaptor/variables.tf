variable "ibmcloud_api_key" {
    sensitive = true
}

variable "ssh_key_name" {
    description = "Name of the SSH public key in VPC used by the Kubernetes worker instance. Must be provided if ssh_key_id is not provided"
    default = null
}

variable "ssh_key_id" {
    description = "ID of the SSH public key in VPC used by the Kubernetes worker instance. Must be provided if ssh_key_name is not provided"
    default = null
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

variable "podvm_image_name" {
    description = "Name of the VPC Custom Image used for the peer pod VM. Must be provided if podvm_image_id is not provided"
    default = null
}

variable "podvm_image_id" {
    description = "ID of the VPC Custom Image used for the peer pod VM. Must be provided if podvm_image_name is not provided"
    default = null
}

variable "region_name" {
    default = "jp-tok"
}

variable "vpc_name" {
    description = "Name of the VPC. Must be provided if vpc_id is not provided"
    default = null
}

variable "vpc_id" {
    description = "ID of the VPC. Must be provided if vpc_name is not provided"
    default = null
}

variable "primary_subnet_name" {
    description = "Name of the primary VPC subnet. Must be provided if primary_subnet_id is not provided"
    default = null
}

variable "primary_subnet_id" {
    description = "ID of the primary VPC subnet. Must be provided if primary_subnet_name is not provided"
    default = null
}

variable "primary_security_group_name" {
    description = "Name of the security group of the primary VPC subnet. Must be provided if primary_subnet_id is not provided"
    default = null
}

variable "primary_security_group_id" {
    description = "ID of the security group of the primary VPC subnet. Must be provided if primary_security_group_name is not provided"
    default = null
}

variable "instance_profile_name" {
    default = "bx2-2x8"
}

variable "use_ibmcloud_test" {
    type = bool
    default = false
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

variable "resource_group_name" {
  description = "The resource group ID where the cloud api adaptor will start peer pod instances"
  default     = null
}

variable "ansible_dir" {
    description = "Subdirectory for Ansible playbook, inventory and vars files"
    default = "./ansible"
}

variable "cri_runtime_endpoint" {
    description = "cri-runtime-endpoint for Ansible playbook, inventory and vars files"
    default = "/run/containerd/containerd.sock"
}
