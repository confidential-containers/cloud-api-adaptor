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

variable "cpu_type" {
  type    = string
  default = "Cascadelake-Server"
}

variable "disk_size" {
  type    = string
  default = "6144"
}

variable "cloud_image_checksum" {
  type    = string
  default = "73c7631c6a48b182e80c7c808d7e3adab3f07ad517fcf5d5eff8f3815306e37e"
}

variable "cloud_image_url" {
  type    = string
  default = "https://alinux3.oss-cn-hangzhou.aliyuncs.com/aliyun_3_x64_20G_nocloud_alibase_20250117.qcow2"
}

variable "memory" {
  type    = string
  default = "2048M"
}

variable "qemu_binary" {
  type    = string
  default = "qemu-system-x86_64"
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
  default = "5m"
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

variable "activation_key" {
  type    = string
  default = env("ACTIVATION_KEY")
}

variable "org_id" {
  type    = string
  default = env("ORG_ID")
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

variable "output_directory" {
  type    = string
  default = "output"
}
