//  BLOCK: source
//  Defines the builder configuration blocks.

//  BLOCK: build
//  Defines the builders to run, provisioners, and post-processors.

locals {
  data_source_content = {
   "/ks.cfg" = templatefile("${abspath(path.root)}/data/ks.pkr.hcl", {
      build_username           = "${var.vm_username}"
      build_password           = "${var.vm_password}"
      vm_guest_os_language     = "${var.vm_guest_os_language}"
      vm_guest_os_keyboard     = "${var.vm_guest_os_keyboard}"
      vm_guest_os_timezone     = "${var.vm_guest_os_timezone}"
      vm_guest_hostname        = "${var.vm_hostname}"
    })
    }
  data_source_command = "inst.ks=cdrom:/ks.cfg"
}

source "vsphere-iso" "rhel" {
// vcenter settings
  vcenter_server          = "${var.vcenter_server}"
  username                = "${var.vcenter_username}"
  datacenter              = "${var.datacenter}"
  password                = "${var.vcenter_password}"
  datastore               = "${var.datastore}"
  cluster	          = "${var.cluster}"
  insecure_connection     = "${var.insecure_connection}"
  guest_os_type           = "${var.vm_guest_os_type}"

// whether to create a template and if yes, the name
  vm_name                 = "${var.template}"
  convert_to_template     = "${var.convert_to_template}"

// ssh user/pass to the guest, same as cloudinit data
  ssh_username            = "${var.vm_username}"
  ssh_password            = "${var.vm_password}"
  ssh_timeout  		  = "${var.ssh_timeout}"

// VM resources
  CPUs                    = var.vm_cpu_count
  RAM                     = var.vm_mem_size
  RAM_reserve_all         = true

  disk_controller_type    = ["pvscsi"]
  storage {
    disk_size             = var.vm_disk_size
    disk_thin_provisioned = var.vm_disk_thin_provisioned
  }

  network_adapters {
    network               = "${var.vm_network_name}"
    network_card          = "${var.vm_interface_name}"
  }

// Attach cloudinit config as a disk
  cd_content              = local.data_source_content
  firmware                = "${var.vm_firmware}"

// iso path and checksum
  iso_url                 = "${var.iso_url}"
  iso_checksum            = "${var.iso_checksum_value}"

// boot command for autoinstall
  boot_wait               = "2s"
  boot_command = [
    "<up>",
    "e",
    "<down><down><end><wait>",
    " text ${local.data_source_command}",
    "<enter><wait><leftCtrlOn>x<leftCtrlOff>"
  ]
}

build {
  sources = ["source.vsphere-iso.rhel"]

 provisioner "file" {
    source      = "./files.tar"
    destination = "/tmp/"
  }

 provisioner "file" {
    source      = "misc-settings.sh"
    destination = "~/misc-settings.sh"
  }

 provisioner "shell" {
    inline = [
      "cd /tmp && sudo tar xf files.tar -C /",
      "rm /tmp/files.tar",
      "sudo bash ~/misc-settings.sh",
      "sudo rm -rf /etc/cloud/cloud-init.disabled"
   ]
  }
}
