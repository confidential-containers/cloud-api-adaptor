
# Red Hat Enterprise Linux Server 9

### Installs from the first attached CD-ROM/DVD on the system.
cdrom

### Performs the kickstart installation in text mode.
text

### Accepts the End User License Agreement.
eula --agreed

### Sets the language to use during installation and the default language to use on the installed system.
lang ${vm_guest_os_language}

### Sets the default keyboard type for the system.
keyboard ${vm_guest_os_keyboard}

### bootproto dhcp and enabled at boot
network --bootproto=dhcp --onboot=yes --hostname=${vm_guest_hostname}

### user with sudo privileges
user --name=${build_username} --plaintext --password=${build_password} --groups=wheel

### firewall is disabled
firewall --disabled

### selinux is permissive
selinux --permissive

### Sets the system time zone.
timezone ${vm_guest_os_timezone}

### Sets how the boot loader should be installed.
bootloader --location=mbr

### Initialize any invalid partition tables found on disks.
zerombr

### Removes partitions from the system, prior to creation of new partitions.
clearpart --all --initlabel

# diskconfig
part /boot --fstype xfs --size=1024 --label=BOOTFS
part /boot/efi --fstype vfat --size=1024 --label=EFIFS
part pv.01 --size=100 --grow
volgroup sysvg --pesize=4096 pv.01
logvol swap --fstype swap --name=lv_swap --vgname=sysvg --size=1024 --label=SWAPFS
logvol / --fstype xfs --name=lv_root --vgname=sysvg --percent=100 --label=ROOTFS


### Modifies the default set of services that will run under the default runlevel.
services --enabled=NetworkManager,sshd,vmtoolsd

### Do not configure X on the installed system.
skipx

### Packages selection.
%packages --ignoremissing --excludedocs
@core
-iwl*firmware
open-vm-tools
cloud-init
%end

### Post-installation commands.
%post
echo "${build_username} ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers.d/${build_username}
sed -i "s/^.*requiretty/#Defaults requiretty/" /etc/sudoers
## Disable cloud-init for packer provisioners to succeed, cloud-init reconfigures ssh keys
touch /etc/cloud/cloud-init.disabled
%end

### Reboot after the installation is complete.
### --eject attempt to eject the media before rebooting.
reboot --eject
