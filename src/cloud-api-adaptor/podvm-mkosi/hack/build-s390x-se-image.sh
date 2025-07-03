#!/bin/bash

set -euo pipefail

ARCH=$(uname -m)
if [ "${ARCH}" != "s390x" ]; then
    echo "Building of SE podvm image is only supported for s390x"
    exit 0
fi
echo "Building SE podvm image for $ARCH"

echo "Finding host key files"
HOST_KEYS_DIR=${HOST_KEYS_DIR:-"/tmp/keys"}
host_keys=""
for i in "${HOST_KEYS_DIR}"/*.crt; do
    [[ -f "$i" ]] || break
    echo "found host key file: \"${i}\""
    host_keys+="-k ${i} "
done
[[ -z $host_keys ]] && echo "Didn't find host key files. Please download host key files to ${HOST_KEYS_DIR} folder " && exit 1

pushd ../podvm-mkosi/build

workdir=$(pwd)
disksize=100G
tmp_img_path="${workdir}/tmp.qcow2"
tmp_nbd=/dev/nbd1
dst_mnt="${workdir}/dst_mnt"

rm "${tmp_img_path}" || true
qemu-img create -f qcow2 "${tmp_img_path}" "${disksize}"

modprobe nbd
qemu-nbd --connect="${tmp_nbd}" "${tmp_img_path}"

echo "Creating boot-se and root partitions"
parted -a optimal "${tmp_nbd}" mklabel gpt \
        mkpart boot-se ext4 1MiB 256MiB \
        mkpart root 256MiB 6400MiB \
        mkpart data 6400MiB ${disksize} \
        set 1 boot on

echo "Waiting for the two partitions to show up"
while true; do
sleep 1
[ -e ${tmp_nbd}p2 ] && break
done

echo "Formatting boot-se partition"
mke2fs -t ext4 -L boot-se ${tmp_nbd}p1
boot_uuid=$(sudo blkid ${tmp_nbd}p1 -s PARTUUID -o value)
export boot_uuid

echo "Setting up encrypted root partition"
mkdir "${workdir}"/rootkeys
mount -t tmpfs rootkeys "${workdir}"/rootkeys
dd if=/dev/random of="${workdir}"/rootkeys/rootkey.bin bs=1 count=64 &> /dev/null
echo YES | sudo cryptsetup luksFormat --type luks2 ${tmp_nbd}p2 --key-file "${workdir}"/rootkeys/rootkey.bin

echo "Setting luks name for root partition"
LUKS_NAME="luks-$(sudo blkid -s UUID -o value ${tmp_nbd}p2)"
export LUKS_NAME

echo "Open luks with name: $LUKS_NAME"
cryptsetup open ${tmp_nbd}p2 "$LUKS_NAME" --key-file "${workdir}"/rootkeys/rootkey.bin
mkfs.ext4 -L "root" /dev/mapper/"${LUKS_NAME}"

echo "Copying the root filesystem"
sudo mkdir -p "${dst_mnt}"
sudo mount /dev/mapper/"$LUKS_NAME" "${dst_mnt}"
sudo mkdir "${dst_mnt}"/boot-se
sudo mount -o norecovery ${tmp_nbd}p1 "${dst_mnt}"/boot-se

# system is the mkosi output directory for system image
src_mnt=system
tar_opts=(--numeric-owner --preserve-permissions --acl --selinux --xattrs --xattrs-include='*' --sparse  --one-file-system)
tar -cf - "${tar_opts[@]}" --sort=none -C "${src_mnt}" . | tar -xf - "${tar_opts[@]}" --preserve-order  -C "${dst_mnt}"

sudo mount -t sysfs sysfs "${dst_mnt}"/sys
sudo mount -t proc proc "${dst_mnt}"/proc
sudo mount --bind /dev "${dst_mnt}"/dev

echo "Adding fstab"
cat <<END > "${dst_mnt}"/etc/fstab
#This file was auto-generated
/dev/mapper/$LUKS_NAME    /        ext4  defaults 1 1
PARTUUID=$boot_uuid    /boot-se    ext4  norecovery 1 2
END

echo "Configure kernel modules"
cat <<END > "${dst_mnt}"/etc/modprobe.d/blacklist-virtio.conf
#do not trust rng from hypervisor
blacklist virtio_rng
END

echo s390_trng >> "${dst_mnt}"/etc/modules

echo "Updating initial ram disk to add luks keyfile"
extra_luks_dir="${workdir}"/luks
mkdir -p "${extra_luks_dir}"/etc/keys
mount -t tmpfs keys "${extra_luks_dir}"/etc/keys

dev_uuid=$(sudo blkid -s UUID -o value "/dev/mapper/$LUKS_NAME")
cp "${workdir}/rootkeys/rootkey.bin" "${extra_luks_dir}/etc/keys/luks-${dev_uuid}.key"
chmod 600 "${extra_luks_dir}/etc/keys/luks-${dev_uuid}.key"

cat <<END > "${extra_luks_dir}"/etc/crypttab
#This file was auto-generated
$LUKS_NAME UUID=$(sudo blkid -s UUID -o value ${tmp_nbd}p2) /etc/keys/luks-$(blkid -s UUID -o value /dev/mapper/"$LUKS_NAME").key luks,discard,initramfs
END
sudo chmod 744 "${extra_luks_dir}/etc/crypttab"

# Update initrd image with mkosi
mkosi --directory ../ --profile production.conf --image initrd --extra-tree "${extra_luks_dir}" --force

umount "${extra_luks_dir}"/etc/keys
rm -rf "${extra_luks_dir}"

echo "Creating SE boot image"
cp initrd.cpio.zst "${dst_mnt}"/boot/initrd.img
cp system.vmlinuz "${dst_mnt}"/boot/vmlinuz

cat <<END > "${dst_mnt}"/etc/zipl.conf
[defaultboot]
default=linux
target=/boot-se

targetbase=${tmp_nbd}
targettype=scsi
targetblocksize=512
targetoffset=2048

[linux]
image = /boot-se/se.img
END

export SE_PARMLINE="root=/dev/mapper/$LUKS_NAME console=ttysclp0 quiet panic=0 rd.shell=0 blacklist=virtio_rng swiotlb=262144 selinux=0 enforcing=0 audit=0 systemd.firstboot=off"
echo "${SE_PARMLINE}" > "${dst_mnt}"/boot/parmfile

sudo -E /usr/bin/genprotimg \
    -i "${dst_mnt}"/boot/vmlinuz \
    -r "${dst_mnt}"/boot/initrd.img \
    -p "${dst_mnt}"/boot/parmfile \
    --no-verify \
    ${host_keys} \
    -o "${dst_mnt}"/boot-se/se.img

# exit and throw an error if no se image was created
[ ! -e "${dst_mnt}"/boot-se/se.img ] && exit 1
# if building the image succeeded wipe /boot
rm -rf "${dst_mnt:?}"/boot/*

echo "Running zipl to prepare boot partition"
sudo chroot "${dst_mnt}" zipl -V --targetbase ${tmp_nbd} \
    --targettype scsi \
    --targetblocksize 512 \
    --targetoffset 2048 \
    --target /boot-se \
    --image /boot-se/se.img

echo "Cleaning luks keyfile"
umount "${workdir}"/rootkeys/ || true
rm -rf "${workdir}"/rootkeys
umount "${dst_mnt}"/boot-se
umount "${dst_mnt}"/dev
umount "${dst_mnt}"/proc
umount "${dst_mnt}"/sys
umount "${dst_mnt}"

echo "Closing encrypted root partition"
cryptsetup close "$LUKS_NAME"

qemu-nbd --disconnect "${tmp_nbd}"

output_img_name="podvm-fedora-s390x-se.qcow2"
qemu-img convert -O qcow2 -c "${tmp_img_path}" "${output_img_name}"
output_img_path=$(realpath "${output_img_name}")
echo "podvm se-image is generated: ${output_img_path}"

popd
