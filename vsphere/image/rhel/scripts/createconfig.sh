#!/bin/bash
ACTION="$1"
CONF="$2"
GREEN='\033[0;32m'
NOCOLOR='\033[0m'
default_guest_config=$(cat <<EOF
/*
    DESCRIPTION:
    RHEL 9.0 variables used by the Packer Plugin for VMware vSphere (vsphere-iso).
*/

// Guest Operating System Metadata
vm_guest_os_language = "en_US"
vm_guest_os_keyboard = "us"
vm_guest_os_timezone = "UTC"
vm_guest_os_type = "rhel9_64Guest"


// Virtual Machine Hardware Settings
vm_firmware              = "efi"
vm_cpu_count             = 2
vm_mem_size              = 2048
// too small a disk size can break the install
vm_disk_size             = 6250
vm_disk_thin_provisioned = true
vm_interface_name         = "vmxnet3"
vm_network_name           = "VM Network"
vm_hostname		  = "podvm"

// Login
vm_username = "peerpod"
vm_password = "peerp0d"

// Removable Media Settings
iso_url            = "RHEL-9.0.0-20220420.0-x86_64-dvd1.iso"
iso_checksum_value = "a387f3230acf87ee38707ee90d3c88f44d7bf579e6325492f562f0f1f9449e89"

// Boot Settings
vm_boot_wait  = "2s"

// Communicator Settings
ssh_port    = 22
ssh_timeout = "15m"
EOF
)
case "$ACTION" in
    "vcenter")
	[ -e "$CONF" ] && echo "$CONF already exists, not overwriting" && exit 0
	read -p "vCenter URL(without https://): " vcenter
	read -p "Datacenter: " datacenter
	read -p "vCenter username: " username
	read -s -p "vCenter password: " password; echo
	read -p "Datastore: " datastore
	read -p "Cluster: " cluster
	read -p "Template Name: " template
	cat << EOF > $CONF
//vCenter configuration settings
vcenter_server = "${vcenter}"
datacenter = "${datacenter}"
vcenter_username = "${username}"
vcenter_password = "${password}"
datastore = "${datastore}"
cluster = "${cluster}"
template = "${template}"
EOF
	printf "${GREEN} Created vcenter config file $CONF with user provided values"
	cat "$CONF"
	printf "${NOCOLOR}"
	;;
    "guest")
	[ -e "$CONF" ] && echo "$CONF already exists, not overwriting" && exit 0
	echo "Writing default guest config to $CONF"
	echo -e "$default_guest_config" > $CONF
	printf  "${GREEN} Created guest config file $CONF with default values \n"
	cat $CONF
	printf "${NOCOLOR}"
	;;
    *)
	echo "invalid input"
	exit 1
	;;
esac
