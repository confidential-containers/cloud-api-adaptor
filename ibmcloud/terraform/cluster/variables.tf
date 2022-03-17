
variable "ibmcloud_api_key" {}
variable "ssh_key_name" {}
variable "cluster_name" {}

variable "region_name" {
    default = "jp-tok"
}

variable "zone_name" {
    default = "jp-tok-2"
}

variable "instance_profile_name" {
    default = "bx2-2x8"
}

variable "image_name" {
    default = "ibm-ubuntu-20-04-3-minimal-amd64-1"
}

variable "vpc_name" {
    default = "tok-vpc"
}

variable "primary_subnet_name" {
    default = "tok-primary-subnet"
}

variable "primary_security_group_name" {
    default = "tok-primary-security-group"
}
