
variable "ibmcloud_api_key" {
    sensitive = true
}

variable "cluster_name" {}
variable "ssh_key_name" {}

variable "image_name" {
    default = "ibm-ubuntu-20-04-3-minimal-amd64-1"
}

variable "instance_profile_name" {
    default = "bx2-2x8"
}

variable "primary_subnet_id" {
    description = "ID of the primary subnet. This or the primary subnet name must be provided"
    default = null
}

variable "primary_subnet_name" {
    description = "Name of the primary subnet. This or the primary subnet ID must be provided"
    default = null
}

variable "primary_security_group_id" {
    description = "ID of the primary security group. This or the primary security group name must be provided"
    default = null
}

variable "primary_security_group_name" {
    description = "Name of the primary security group. This or the primary security group ID must be provided"
    default = null
}

variable "region_name" {
    default = "jp-tok"
}

variable "ssh_pub_key" {
    default = ""
}

variable "vpc_id" {
    description = "ID of the VPC. This or the VPC name must be provided"
    default = null
}

variable "vpc_name" {
    description = "Name of the VPC. This or the VPC ID must be provided"
    default = null
}

variable "zone_name" {
    default = "jp-tok-2"
}

variable "ansible_dir" {
    description = "Subdirectory for Ansible playbook, inventory and vars files"
    default = "./ansible"
}

variable "scripts_dir" {
    description = "Subdirectory for shell scripts"
    default = "./scripts"
}