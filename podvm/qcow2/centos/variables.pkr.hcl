// variables.pkr.hcl

// For those variables that you don't provide a default for, you must
// set them from the command line, a var-file, or the environment.

variable "cloud_init_image" {
  type    = string
  default = "cloud-init.img"
}

variable "cpus" {
  type    = string
  default = "2"
}

variable "disk_size" {
  type    = string
  # This is the default virtual size of CentOS cloud image (qcow2)
  default = "10240"
}

variable "cloud_image_checksum" {
  type    = string
  default = "b80ca9ccad8715aa1aeee582906096d3190b91cebe05fcb3ac7cb197fd68a7fe"
}

variable "cloud_image_url" {
  type    = string
  default = "https://cloud.centos.org/centos/8-stream/x86_64/images/CentOS-Stream-GenericCloud-8-20200113.0.x86_64.qcow2"
}

variable "memory" {
  type    = string
  default = "2048M"
}

variable "ssh_password" {
  type    = string
  default = "PeerP0d"
}

variable "ssh_username" {
  type    = string
  default = "peerpod"
}

variable "qemu_image_name" {
  type    = string
  default = "peer-pod"
}
