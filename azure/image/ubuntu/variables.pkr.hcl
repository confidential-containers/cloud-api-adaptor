//  variables.pkr.hcl

// For those variables that you don't provide a default for, you must
// set them from the command line, a var-file, or the environment.

variable "az_image_name" {
  type    = string
  default = "peer-pod-vmimage"
}

// shared gallery name
variable "az_gallery_name" {
  type    = string
  default = "caaubntcvmsGallery"
}

// shared gallery image name
variable "az_gallery_image_name" {
  type    = string
  default = "cc-image"
}

// shared gallery image version
variable "az_gallery_image_version" {
  type    = string
  default = "0.0.1"
}

// instance type
variable "vm_size" {
  type    = string
  default = "Standard_D2as_v5"
}

variable "resource_group" {
  type = string
}

variable "client_id" {
  type = string
  # This can be empty when using local authentication enabled by setting `use_azure_cli_auth` to true.
  default   = ""
  sensitive = true
}

variable "client_secret" {
  type = string
  # This can be empty when using local authentication enabled by setting `use_azure_cli_auth` to true.
  default   = ""
  sensitive = true
}

variable "subscription_id" {
  type = string
  # This can be empty when using local authentication enabled by setting `use_azure_cli_auth` to true.
  default   = ""
  sensitive = true
}

variable "tenant_id" {
  type = string
  # This can be empty when using local authentication enabled by setting `use_azure_cli_auth` to true.
  default   = ""
  sensitive = true
}

variable "use_azure_cli_auth" {
  type    = bool
  default = false
}

variable "ssh_username" {
  type    = string
  default = "peerpod"
}

variable "publisher" {
  type    = string
  default = "Canonical"
}

variable "offer" {
  type    = string
  default = "0001-com-ubuntu-confidential-vm-jammy"
}

variable "sku" {
  type    = string
  default = "22_04-lts-cvm"
}

variable "podvm_distro" {
  type    = string
  default = env("PODVM_DISTRO")
}

variable "cloud_provider" {
  type    = string
  default = env("CLOUD_PROVIDER")
}

variable "plan_name" {
  type    = string
  default = ""
}

variable "plan_product" {
  type    = string
  default = ""
}

variable "plan_publisher" {
  type    = string
  default = ""
}

