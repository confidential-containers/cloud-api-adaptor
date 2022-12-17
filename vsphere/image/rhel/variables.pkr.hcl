/*
    DESCRIPTION:
    Ubuntu Server 20.04 LTS variables using the Packer Builder for VMware vSphere (vsphere-iso).
*/

// vSphere Credentials

variable "vcenter_server" {
  type        = string
  description = "The fully qualified domain name or IP address of the vCenter Server instance"
  default = null
}

variable "vcenter_username" {
  type        = string
  description = "vCenter login, do not use vsphere credentials here!"
  default = null
}

variable "datacenter" {
  type        = string
  description = "The name of the target datacenter"
  default = null
}

variable "vcenter_password" {
  type        = string
  description = "vCenter password"
  default = null
}

variable "datastore" {
  type        = string
  description = "The name of the target datastore"
  default = null
}

variable "cluster" {
  type        = string
  description = "The name of the target cluster"
  default = null
}

variable "template" {
  type        = string
  description = "The name of the template to use"
  default = null
}

variable "vm_guest_os_type" {
  type        = string
  description = "Guest OS type"
  default = null
}

variable "vm_guest_os_language" {
  type        = string
  description = "Guest OS language"
  default = "en_US"
}

variable "vm_guest_os_keyboard" {
  type        = string
  description = "Guest OS keyboard"
  default = "us"
}

variable "vm_guest_os_timezone" {
  type        = string
  description = "Guest OS timezone"
  default = "UTC"
}

variable "vm_firmware" {
  type        = string
  description = "Guest os bios - legacy or efi"
  default = "efi"
}

variable "vm_cpu_count" {
  type        = number
  description = "Number of guest cpus"
}

variable "vm_mem_size" {
  type        = number
  description = "Guest memory"
}

variable "vm_disk_size" {
  type        = number
  description = "Guest disk size"
}

variable "vm_disk_thin_provisioned" {
  type        = string
  description = "Thin provisioning"
  default = "true"
}

variable "vm_interface_name" {
  type        = string
  description = "interface name"
  default = null
}

variable "vm_network_name" {
  type        = string
  description = "Network name"
  default = "VM Network"
}

variable "iso_url" {
  type        = string
  description = "URL of installation iso"
  default = null
}

variable "iso_checksum_value" {
  type        = string
  description = "Checksum of installation iso"
  default = null
}

variable "vm_boot_wait" {
  type        = string
  description = "Wait time for guest keyboard input"
  default = "2s"
}

variable "ssh_port" {
  type        = number
  description = "ssh port"
  default = 22
}

variable "ssh_timeout" {
  type        = string
  description = "time to wait for ssh to be alive in the guest"
}

variable "convert_to_template" {
  type        = bool
  description = "time to wait for ssh to be alive in the guest"
  default = true
}

variable "insecure_connection" {
  type        = bool
  description = "time to wait for ssh to be alive in the guest"
  default = true
}

variable "vm_hostname" {
  type        = string
  description = "name of the podvm guest, by default time is suffixed"
  default = "podvm"
}

variable "vm_username" {
  type        = string
  description = "podvm username"
  default = "peerpod"
}

variable "vm_password" {
  type        = string
  description = "podvm password"
  default = "peerp0d"
}
