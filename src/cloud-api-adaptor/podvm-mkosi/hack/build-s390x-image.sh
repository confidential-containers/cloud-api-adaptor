#!/bin/bash

set -euo pipefail

# pushd ../podvm-mkosi/build

workdir=.
tmp_img_path="${workdir}/tmp.qcow2"
tmp_nbd=/dev/nbd1
dst_mnt=./dst_mnt
disksize=100G

qemu-img create -f qcow2 "${tmp_img_path}" "${disksize}"

modprobe nbd
qemu-nbd --connect="${tmp_nbd}" "${tmp_img_path}"

# Partition and format
parted -a optimal "${tmp_nbd}" mklabel gpt \
        mkpart boot ext4 1MiB 256MiB \
        mkpart system 256MiB "${disksize}" \
        set 1 boot on

echo "Waiting for the two nbd partitions to show up"
while true; do
sleep 1
[[ -e "${tmp_nbd}"p2 ]] && break
done

mke2fs -t ext4 -L boot "${tmp_nbd}"p1
boot_uuid=$(blkid "${tmp_nbd}"p1 -s PARTUUID -o value)

mke2fs -t ext4 -L system "${tmp_nbd}"p2
system_uuid=$(blkid "${tmp_nbd}"p2 -s PARTUUID -o value)

# Copy files
mkdir -p "${dst_mnt}"
mount "${tmp_nbd}p2" "${dst_mnt}"

mkdir -p "${dst_mnt}"/boot
mount -o norecovery "${tmp_nbd}"p1 "${dst_mnt}"/boot

cp initrd.cpio.zst "${dst_mnt}"/boot/initrd.img
cp system.vmlinuz "${dst_mnt}"/boot/vmlinuz

src_mnt=system
tar_opts=(--numeric-owner --preserve-permissions --acl --selinux --xattrs --xattrs-include='*' --sparse)
tar -cf - "${tar_opts[@]}" --sort=none -C "${src_mnt}" . | tar -xf - "${tar_opts[@]}" --preserve-order  -C "${dst_mnt}"

cat <<END > "${dst_mnt}/etc/fstab"
#This file was auto-generated
PARTUUID=${system_uuid}   /        ext4  defaults 1 1
PARTUUID=${boot_uuid}     /boot    ext4  norecovery 1 2
END

mount -t sysfs sysfs "${dst_mnt}/sys"
mount -t proc proc "${dst_mnt}/proc"
mount --bind /dev "${dst_mnt}/dev"

# generate bootloader
chroot "${dst_mnt}" zipl -V --targetbase "${tmp_nbd}" \
    --targettype scsi \
    --targetblocksize 512 \
    --targetoffset 2048 \
    --target /boot \
    --image /boot/vmlinuz \
    --ramdisk /boot/initrd.img \
    --parameters "root=LABEL=system selinux=0 enforcing=0 audit=0 systemd.firstboot=off"

umount "${dst_mnt}/dev"
umount "${dst_mnt}/proc"
umount "${dst_mnt}/sys"

umount "${dst_mnt}/boot"
umount "${dst_mnt}"

qemu-nbd --disconnect "${tmp_nbd}"

output_img_name="podvm-fedora-s390x.qcow2"
qemu-img convert -O qcow2 -c "${tmp_img_path}" "${output_img_name}"
chmod 644 "${output_img_name}"

output_img_path=$(realpath "${output_img_name}")
echo "podvm image is generated: ${output_img_path}"

popd
