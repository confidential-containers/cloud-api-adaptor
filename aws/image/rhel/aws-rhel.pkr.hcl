packer {
  required_plugins {
    amazon = {
      version = ">= 0.0.2"
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
      name                = "RHEL-8.7.0_HVM-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
      architecture        = "x86_64"
    }

    most_recent = true
    owners      = ["309956199498"]
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
    source      = "copy-files.sh"
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
    inline = [
      "sudo -E bash ~/misc-settings.sh"
    ]
  }
}
