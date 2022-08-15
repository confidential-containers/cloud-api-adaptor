##############################################################################
# Input Variables
##############################################################################

# Resource Group Variables
variable "resource_group_name" {
  type        = string
  description = "The resource group ID where the environment will be created"
  default     = null
}

variable "ibmcloud_api_key" {
  description = "API key to login to IBM Cloud"
  type        = string
  sensitive   = true
}

variable "region_name" {
  description = "Name of the Region to deploy in to"
  type        = string
  default     = "jp-tok"
}

variable "cos_bucket_name" {
  description = "Name of the COS bucket to create"
  type        = string
}

variable "cos_service_instance_name" {
  description = "Name of the COS instance to create"
  type        = string
  default     = "cos-image-instance"
}

variable "cos_bucket_region" {
  description = "Name of the region in which to create the COS instance"
  type        = string
  default     = ""
}
