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
  default = "6144"
}

variable "cloud_image_checksum" {
  type    = string
  default = "d96622d77bcbab5526fd42e7d933ee851d239327946992a018b0bfc9fad777e7"
}

variable "cloud_image_url" {
  type    = string
  default = "https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img"
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
