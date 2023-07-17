//  variables.pkr.hcl

// For those variables that you don't provide a default for, you must
// set them from the command line, a var-file, or the environment.

variable "gce_image_name" {
  type    = string
  default = "peer-pod-gceimage"
}

variable "machine_type" {
  type    = string
  default = "e2-medium"
}

variable "zone" {
  type = string
}

variable "network" {
  type    = string
  default = null
}

variable "project_id" {
  type    = string
  default = env("GCP_PROJECT_ID")
}

variable "podvm_distro" {
  type    = string
  default = env("PODVM_DISTRO")
}

variable "cloud_provider" {
  type    = string
  default = env("CLOUD_PROVIDER")
}

variable "ssh_username" {
  type    = string
  default = "ubuntu"
}

