source "qemu" "centos" {
  boot_command      = ["<enter>"]
  disk_compression  = true
  disk_image        = true
  disk_size         = "${var.disk_size}"
  format            = "qcow2"
  headless          = true
  iso_checksum      = "${var.cloud_image_checksum}"
  iso_url           = "${var.cloud_image_url}"
  output_directory  = "output"
  qemuargs          = [["-m", "${var.memory}"], ["-smp", "cpus=${var.cpus}"], ["-cdrom", "${var.cloud_init_image}"], ["-serial", "mon:stdio"]]
  ssh_password      = "${var.ssh_password}"
  ssh_port          = 22
  ssh_username      = "${var.ssh_username}"
  ssh_wait_timeout  = "300s"
  vm_name           = "${var.qemu_image_name}"
  shutdown_command  = "sudo shutdown -h now" 
  qemu_binary       = "/usr/libexec/qemu-kvm"
}

build {
  sources = ["source.qemu.centos"]

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
    inline = [
      "sudo bash ~/misc-settings.sh"
    ]
  }

}
