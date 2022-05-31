
variable "ibmcloud_api_key" {
    sensitive = true
}

variable "floating_ip_name" {
    default = "tok-gateway-ip"
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

variable "zone_name" {
    default = "jp-tok-2"
}