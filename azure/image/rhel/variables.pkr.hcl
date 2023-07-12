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
  type = string
}

variable "offer" {
  type = string
}

variable "sku" {
  type = string
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
