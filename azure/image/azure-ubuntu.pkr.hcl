source "azure-arm" "ubuntu" {
  client_id       = "${var.client_id}"
  client_secret   = "${var.client_secret}"
  subscription_id = "${var.subscription_id}"
  tenant_id       = "${var.tenant_id}"

  vm_size                           = "${var.vm_size}"
  location                          = "${var.location}"
  os_type                           = "Linux"
  image_publisher                   = "Canonical"
  image_offer                       = "0001-com-ubuntu-minimal-jammy"
  image_sku                         = "minimal-22_04-lts"
  managed_image_name                = "${var.az_image_name}"
  managed_image_resource_group_name = "${var.resource_group}"

}

build {
  name = "peer-pod-ubuntu"
  sources = [
    "source.azure-arm.ubuntu"
  ]

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
    execute_command = "chmod +x {{ .Path }}; {{ .Vars }} sudo -E sh '{{ .Path }}'"
    inline = [
      "/usr/sbin/waagent -force -deprovision+user && export HISTSIZE=0 && sync"
    ]
    inline_shebang = "/bin/sh -x"
  }


}
