
variable "ibmcloud_api_key" {}

variable "region_name" {
    default = "jp-tok"
}

variable "vpc_name" {
    default = "tok-vpc"
}

variable "zone_name" {
    default = "jp-tok-2"
}

variable "public_gateway_name" {
    default = "tok-gateway"
}

variable "floating_ip_name" {
    default = "tok-gateway-ip"
}
variable "primary_subnet_name" {
    default = "tok-primary-subnet"
}

variable "primary_security_group_name" {
    default = "tok-primary-security-group"
}
