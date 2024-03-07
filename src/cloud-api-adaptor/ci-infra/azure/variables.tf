# Note: Using `ver` because `version` is a built-in variable.
variable "ver" {
  type        = number
  description = "Monotonically increasing number to track version of infrastructure"
}

variable "ci_rg" {
  type        = string
  default     = "azure-caa-ci"
  description = "Resource group for CI resources"
}

variable "container_registry" {
  type        = string
  default     = "azurecaa"
  description = "Container registry for holding CAA images"
}

variable "image_gallery" {
  type    = string
  default = "podvm_gallery"
}

variable "image_definition" {
  type    = string
  default = "podvm_image"
}

variable "aks_rg" {
  type        = string
  default     = "azure-caa-ci-aks"
  description = "Resource group for holding AKS resources"
}

variable "location" {
  type        = string
  default     = "eastus"
  description = "Location for all resources"
}

variable "gh_action_user_identity" {
  type        = string
  default     = "ghactions_user"
  description = "User assigned identity for the GH runner"
}

variable "gh_action_federated_credential" {
  type        = string
  default     = "ghactions_credential"
  description = "Federated credential for the GH runner"
}

variable "gh_repo" {
  type        = string
  description = "GitHub repository that has permissions to run workloads on Azure. The value should be in the format `orgName/repoName`"
  default     = "confidential-containers/cloud-api-adaptor"
}
