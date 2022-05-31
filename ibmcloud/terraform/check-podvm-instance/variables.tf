variable "ibmcloud_api_key" {
    sensitive = true
}

variable "region_name" {
    default = "jp-tok"
}

variable "podvm_image_name" {
    description = "Name of the VPC Custom Image used for the peer pod VM. Must be provided if podvm_image_id is not provided"
    default = null
}

variable "podvm_image_id" {
    description = "ID of the VPC Custom Image used for the peer pod VM. Must be provided if podvm_image_name is not provided"
    default = null
}

variable "virtual_server_instances" {
    description = "List of Virtual Server instances currently running in the VPC. Obtained by calling data \"ibm_is_instances\" \"instances\""
}
