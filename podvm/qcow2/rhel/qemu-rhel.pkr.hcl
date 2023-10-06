locals {
  machine_type = "${var.os_arch}" == "x86_64" && "${var.is_uefi}" ? "q35" : "${var.machine_type}"
  use_pflash   = "${var.os_arch}" == "x86_64" && "${var.is_uefi}" ? "true" : "false"
  firmware     = "${var.os_arch}" == "x86_64" && "${var.is_uefi}" ? "${var.uefi_firmware}" : ""
}

source "qemu" "rhel" {
  disable_vnc      = true
  disk_compression = true
  disk_image       = true
  disk_size        = "${var.disk_size}"
  format           = "qcow2"
  headless         = true
  iso_checksum     = "${var.cloud_image_checksum}"
  iso_url          = "${var.cloud_image_url}"
  output_directory = "output"
  qemuargs         = [["-m", "${var.memory}"], ["-smp", "cpus=${var.cpus}"], ["-cdrom", "${var.cloud_init_image}"], ["-serial", "mon:stdio"], ["-cpu", "Cascadelake-Server"]]
  ssh_password     = "${var.ssh_password}"
  ssh_port         = 22
  ssh_username     = "${var.ssh_username}"
  ssh_timeout      = "${var.ssh_timeout}"
  boot_wait        = "${var.boot_wait}"
  vm_name          = "${var.qemu_image_name}"
  shutdown_command = "sudo shutdown -h now"
  machine_type     = "${local.machine_type}"
  use_pflash       = "${local.use_pflash}"
  firmware         = "${local.firmware}"
}

build {
  sources = ["source.qemu.rhel"]

  provisioner "shell-local" {
    command = "tar cf toupload/files.tar files"
  }

  provisioner "file" {
    source      = "./toupload"
    destination = "/tmp/"
  }

  provisioner "shell" {
    inline = [
      "cd /tmp && tar xf toupload/files.tar",
      "rm toupload/files.tar"
    ]
  }

  provisioner "file" {
    source      = "qcow2/copy-files.sh"
    destination = "~/copy-files.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    inline = [
      "sudo bash ~/copy-files.sh"
    ]
  }

  # relabel copied files right after copy-files.sh
  # to prevent other commands from failing
  provisioner "file" {
    source      = "qcow2/selinux_relabel.sh"
    destination = "~/selinux_relabel.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    inline = [
      "sudo bash ~/selinux_relabel.sh"
    ]
  }

  provisioner "file" {
    source      = "qcow2/misc-settings.sh"
    destination = "~/misc-settings.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    environment_vars = [
      "CLOUD_PROVIDER=${var.cloud_provider}",
      "PODVM_DISTRO=${var.podvm_distro}",
      "DISABLE_CLOUD_CONFIG=${var.disable_cloud_config}"
    ]
    inline = [
      "sudo -E bash ~/misc-settings.sh"
    ]
  }
}
