//  variables.pkr.hcl

// For those variables that you don't provide a default for, you must
// set them from the command line, a var-file, or the environment.

variable "ami_name" {
  type    = string
  default = "peer-pod-ami"
}

variable "instance_type" {
  type    = string
  default = "t3.small"
}

variable "region" {
  type    = string
}

variable "vpc_id" {
  type = string
  default = null
}

variable "subnet_id" {
  type = string
  default = null
}
