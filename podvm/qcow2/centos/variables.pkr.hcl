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
  type = string
  # This is the default virtual size of CentOS cloud image (qcow2)
  default = "10240"
}

variable "cloud_image_checksum" {
  type    = string
  default = "d12bb6934dd207e242d6aa13f6a4ca4969449c14c3bbdd88a5ce5f5203597a40"
}

variable "cloud_image_url" {
  type    = string
  default = "https://cloud.centos.org/centos/9-stream/x86_64/images/CentOS-Stream-GenericCloud-9-20231002.0.x86_64.qcow2"
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

variable "ssh_timeout" {
  type    = string
  default = "15m"
}

variable "qemu_image_name" {
  type    = string
  default = "peer-pod"
}

variable "podvm_distro" {
  type    = string
  default = env("PODVM_DISTRO")
}

variable "cloud_provider" {
  type    = string
  default = env("CLOUD_PROVIDER")
}

variable "machine_type" {
  type    = string
  default = "pc"
}

variable "os_arch" {
  type    = string
  default = "x86_64"
}

variable "is_uefi" {
  type    = bool
  default = false
}

variable "uefi_firmware" {
  type    = string
  default = "/usr/share/edk2/ovmf/OVMF_CODE.cc.fd"
}

variable "boot_wait" {
  type    = string
  default = "10s"
}

variable "disable_cloud_config" {
  type    = string
  default = env("DISABLE_CLOUD_CONFIG")
}