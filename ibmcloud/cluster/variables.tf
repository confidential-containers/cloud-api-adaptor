#
# (C) Copyright IBM Corp. 2023.
# SPDX-License-Identifier: Apache-2.0
#

variable "ibmcloud_api_key" {
  sensitive = true
}

variable "cluster_name" {
  default = "caa-cluster"
}

variable "ssh_key_name" {}

variable "ssh_pub_key" {
  default = ""
}

variable "vpc_name" {
  type        = string
  description = "(optional) Specify existing VPC name. If none is provided, it will create a new VPC named {cluster_name}-vpc"
  default     = ""
}

variable "subnet_name" {
  type        = string
  description = "(optional) Specify existing subnet name. If none is provided, it will create a new subnet named {cluster_name}-subnet. This must be provided if vpc_name has been set"
  default     = ""
}

# amd64: ibm-ubuntu-20-04-3-minimal-amd64-1
# s390x: ibm-ubuntu-20-04-2-minimal-s390x-1
variable "node_image" {
  default = "ibm-ubuntu-20-04-2-minimal-s390x-1"
}

# amd64: bx2-2x8
# s390x: bz2-2x8
variable "node_profile" {
  default = "bz2-2x8"
}

variable "nodes" {
  type    = number
  default = 2
}

variable "region" {
  default = "jp-tok"
}

variable "zone" {
  default = "jp-tok-2"
}

variable "containerd_version" {
  default = "1.7.0-beta.3"
}

variable "kube_version" {
  default = "1.28"
}
