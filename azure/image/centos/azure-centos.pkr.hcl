source "azure-arm" "centos" {

  plan_info {
    plan_name      = "${var.plan_name}"
    plan_product   = "${var.plan_product}"
    plan_publisher = "${var.plan_publisher}"
  }

  use_azure_cli_auth = "${var.use_azure_cli_auth}"
  client_id          = "${var.client_id}"
  client_secret      = "${var.client_secret}"
  subscription_id    = "${var.subscription_id}"
  tenant_id          = "${var.tenant_id}"

  vm_size                           = "${var.vm_size}"
  os_type                           = "Linux"
  image_publisher                   = "${var.publisher}"
  image_offer                       = "${var.offer}"
  image_sku                         = "${var.sku}"
  managed_image_name                = "${var.az_image_name}"
  managed_image_resource_group_name = "${var.resource_group}"
  build_resource_group_name         = "${var.resource_group}"
}

build {
  name = "peer-pod-centos"
  sources = [
    "source.azure-arm.centos"
  ]

  provisioner "shell-local" {
    command = "tar cf toupload/files.tar -C ../../podvm files"
  }

  provisioner "file" {
    source      = "toupload"
    destination = "/tmp/"
  }

  provisioner "shell" {
    inline = [
      "cd /tmp && tar xf toupload/files.tar",
      "rm toupload/files.tar"
    ]
  }

  provisioner "file" {
    source      = "copy-files.sh"
    destination = "~/copy-files.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    inline = [
      "sudo bash ~/copy-files.sh"
    ]
  }

  provisioner "shell" {
    inline = [
      "sudo useradd -m -s /bin/bash ${var.ssh_username}"
    ]
  }

  # relabel copied files right after copy-files.sh
  # to prevent other commands from failing
  provisioner "file" {
    source      = "selinux_relabel.sh"
    destination = "~/selinux_relabel.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    inline = [
      "sudo bash ~/selinux_relabel.sh"
    ]
  }

  provisioner "file" {
    source      = "misc-settings.sh"
    destination = "~/misc-settings.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    environment_vars = [
      "CLOUD_PROVIDER=${var.cloud_provider}",
      "PODVM_DISTRO=${var.podvm_distro}",
    ]
    inline = [
      "sudo -E bash ~/misc-settings.sh"
    ]
  }

  provisioner "shell" {
    execute_command = "chmod +x {{ .Path }}; {{ .Vars }} sudo -E sh '{{ .Path }}'"
    inline = [
      "/usr/sbin/waagent -force -deprovision+user && export HISTSIZE=0 && sync"
    ]
    inline_shebang = "/bin/sh -x"
  }
}
