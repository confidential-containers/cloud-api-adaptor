packer {
  required_plugins {
    amazon = {
      version = "= 1.3.1"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

source "amazon-ebs" "rhel" {
  ami_name      = "${var.ami_name}"
  instance_type = "${var.instance_type}"
  region        = "${var.region}"
  vpc_id        = "${var.vpc_id}"
  subnet_id     = "${var.subnet_id}"
  source_ami_filter {
    filters = {
      name                = "RHEL-9.4.0_HVM-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
      architecture        = "x86_64"
    }

    most_recent = true
    owners      = ["309956199498"]
  }
  ami_block_device_mappings {
    device_name           = "/dev/sda1"
    delete_on_termination = "true"
    volume_size           = "${var.volume_size}"
  }

  ssh_username = "ec2-user"
}

build {
  name = "peer-pod-rhel"
  sources = [
    "source.amazon-ebs.rhel"
  ]

  provisioner "shell-local" {
    command = "tar cf toupload/files.tar -C ../../podvm files"
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
    source      = "${var.config_script_src}/copy-files.sh"
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
    source      = "${var.config_script_src}/selinux_relabel.sh"
    destination = "~/selinux_relabel.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    inline = [
      "sudo bash ~/selinux_relabel.sh"
    ]
  }

  provisioner "file" {
    source      = "${var.config_script_src}/misc-settings.sh"
    destination = "~/misc-settings.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    environment_vars = [
      "CLOUD_PROVIDER=${var.cloud_provider}",
      "PODVM_DISTRO=${var.podvm_distro}",
      "DISABLE_CLOUD_CONFIG=${var.disable_cloud_config}",
      "FORWARDER_PORT=${var.forwarder_port}"
    ]
    inline = [
      "sudo -E bash ~/misc-settings.sh"
    ]
  }

  # Addons
  # To avoid multiple conditionals, copying the entire addons directory
  # Individual addons are installed based on environment_vars by setup_addons.sh
  provisioner "shell-local" {
    command = "tar cf toupload/addons.tar -C ../../podvm addons"
  }

  provisioner "file" {
    source      = "toupload"
    destination = "/tmp/"
  }

  provisioner "shell" {
    inline = [
      "cd /tmp && tar xf toupload/addons.tar",
      "rm toupload/addons.tar"
    ]
  }

  provisioner "file" {
    source      = "${var.addons_script_src}/setup_addons.sh"
    destination = "~/setup_addons.sh"
  }

  provisioner "shell" {
    remote_folder = "~"
    environment_vars = [
      "CLOUD_PROVIDER=${var.cloud_provider}",
      "PODVM_DISTRO=${var.podvm_distro}",
      "ENABLE_NVIDIA_GPU=${var.enable_nvidia_gpu}"
    ]
    inline = [
      "sudo -E bash ~/setup_addons.sh"
    ]
  }

}
