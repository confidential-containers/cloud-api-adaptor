variable "ibmcloud_api_key" {
    sensitive = true
}

variable "ibmcloud_user_id" {
  description = "User ID that owns the provided IBM Cloud API key"
}

variable "cluster_name" {}

variable "ssh_key_name" {}

variable "ssh_pub_key" {
    default = ""
}

variable "podvm_image_name" {}

variable "cos_bucket_name" {}

variable "cos_service_instance_name" {
    default = "cos-image-instance"
}

variable "floating_ip_name" {
    default = "tok-gateway-ip"
}

variable "image_name" {
    default = "ibm-ubuntu-20-04-3-minimal-amd64-1"
}

variable "instance_profile_name" {
    default = "bx2-2x8"
}

variable "primary_security_group_name" {
    default = "tok-primary-security-group"
}

variable "primary_subnet_name" {
    default = "tok-primary-subnet"
}

variable "public_gateway_name" {
    default = "tok-gateway"
}

variable "region_name" {
    default = "jp-tok"
}

variable "vpc_name" {
    default = "tok-vpc"
}

variable "use_ibmcloud_test" {
    type = bool
    default = false
}

variable "zone_name" {
    default = "jp-tok-2"
}

variable "skip_verify_console" {
    description = "Set to true to skip checking the console output after starting a virtual server instance using the built pod VM image"
    type = bool
    default = true
}