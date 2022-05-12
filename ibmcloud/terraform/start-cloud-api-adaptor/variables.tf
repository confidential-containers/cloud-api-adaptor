variable "ibmcloud_api_key" {
    sensitive = true
}
variable "ssh_key_name" {}
variable "cluster_name" {}
variable "podvm_image_name" {}

variable "region_name" {
    default = "jp-tok"
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

variable "instance_profile_name" {
    default = "bx2-2x8"
}

variable "use_ibmcloud_test" {
    type = bool
    default = false
}
