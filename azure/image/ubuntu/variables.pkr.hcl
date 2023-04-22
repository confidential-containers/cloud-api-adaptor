//  variables.pkr.hcl

// For those variables that you don't provide a default for, you must
// set them from the command line, a var-file, or the environment.

variable "az_image_name" {
  type    = string
  default = "peer-pod-vmimage"
}

// instance type
variable "vm_size" {
  type    = string
  default = "Standard_A2_v2"
}

// region
variable "location" {
  type = string
  default = "eastus"
}

variable "resource_group" {
  type = string
}

variable "client_id" {
  type = string
}

variable "client_secret" {
  type = string
}

variable "subscription_id" {
  type = string
}

variable "tenant_id" {
  type = string
}

variable "ssh_username" {
  type = string
  default = "peerpod"
}

variable "publisher" {
  type = string
  default = "Canonical"
}

variable "offer" {
  type = string
  default = "0001-com-ubuntu-minimal-jammy"
}

variable "sku" {
  type = string
  default = "minimal-22_04-lts"
}

variable "podvm_distro" {
  type    = string
  default = env("PODVM_DISTRO")
}

variable "cloud_provider" {
  type    = string
  default = env("CLOUD_PROVIDER")
}

