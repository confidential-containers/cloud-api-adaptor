variable "ibmcloud_api_key" {
    sensitive = true
}

variable "region_name" {
    default = "jp-tok"
}

variable "podvm_image_name" {}

variable "vpc_name" {
    default = "tok-vpc"
}