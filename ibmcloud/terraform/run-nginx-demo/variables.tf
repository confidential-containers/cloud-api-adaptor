variable "ibmcloud_api_key" {
    sensitive = true
}

variable "cluster_name" {}

variable "region_name" {
    default = "jp-tok"
}