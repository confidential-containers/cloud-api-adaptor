variable "ibmcloud_api_key" {
    sensitive = true
}
variable "cluster_name" {}

variable "ibmcloud_user_id" {
  description = "User ID that owns the provided IBM Cloud API key"
}

variable "region_name" {
    default = "jp-tok"
}

variable "vpc_name" {
    default = "tok-vpc"
}

variable "primary_subnet_name" {
    default = "tok-primary-subnet"
}

variable "cos_service_instance_name" {
    default = null
}

variable "cos_bucket_name" {
    default = null
}

variable "use_ibmcloud_test" {
    type = bool
    default = false
}