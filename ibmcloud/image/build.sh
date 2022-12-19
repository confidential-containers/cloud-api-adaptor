#!/bin/bash
#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit -o pipefail -o nounset

cd "$(dirname "${BASH_SOURCE[0]}")"

function usage() {
    echo "Usage: $0 --base <base image path> --output <output image path> --root <files dir path> [--packages <package names>]"
}

declare -a packages

workdir=.

while (( $# )); do
    case "$1" in
        --base)     base_img_path=$2 ;;
        --output)   dst_img_path=$2 ;;
        --root)     files_dir=$2 ;;
        --packages) IFS=', ' read -a packages <<< "$2" ;;
        --workdir)  workdir=$2 ;;
        --help)     usage; exit 0 ;;
        *)          usage 1>&2; exit 1;;
    esac
    shift 2
done

if [[ -z "${base_img_path-}" || -z "${dst_img_path-}" || -z "${files_dir-}" ]]; then
    usage 1>&2
    exit 1
fi

SE_BOOT=${SE_BOOT:-0}

if [ "${SE_BOOT}" = "1" ]; then
    if [[ -z "${HOST_KEYS_DIR-}" ]]; then
        echo "HOST_KEYS_DIR is missed" 1>&2
        echo "CLOUD_PROVIDER=ibmcloud SE_BOOT=1 HOST_KEYS_DIR=<host keys directory> make build"
        exit 1
    fi
    umount ./rootkeys/ || true
    rm -rf ./rootkeys/
fi

base_img_path=$(realpath "$base_img_path")
src_img_path="$workdir/src.qcow2"
tmp_img_path="$workdir/tmp.qcow2"

src_nbd=/dev/nbd0
tmp_nbd=/dev/nbd1

src_mnt=./src_mnt
dst_mnt=./dst_mnt

disksize=100G

if [[ -e "$dst_img_path" ]]; then
    echo "Error: image file already exists: $dst_img_path" 1>&2
    exit 1
fi

function cleanup () {
    msg=$1
    if [ "${SE_BOOT}" = "1" ]; then
        for mnt in "$dst_mnt/boot-se" "$dst_mnt/etc/keys" "$dst_mnt/sys"; do
            mountpoint -q "$mnt" && umount "$mnt" || true
            [[ -d "$mnt" ]] && rmdir "$mnt" 2> /dev/null || true
        done
        mountpoint -q ./rootkeys/ && umount ./rootkeys/ || true
        [[ -d ./rootkeys/ ]] && rm -rf ./rootkeys/
    fi
    for mnt in "$src_mnt/run" "$src_mnt/dev/pts" "$src_mnt/dev" \
               "$src_mnt/proc" "$src_mnt/sys" "$src_mnt" \
               "$dst_mnt/dev" "$dst_mnt/proc" "$dst_mnt"; do
        mountpoint -q "$mnt" && umount "$mnt"
        [[ -d "$mnt" ]] && rmdir "$mnt" 2> /dev/null || true
    done
    sleep 1
    [ -n "${LUKS_NAME:-}" ] && (cryptsetup close $LUKS_NAME 2> /dev/null || true)
    qemu-nbd --disconnect "$src_nbd"
    qemu-nbd --disconnect "$tmp_nbd"
    kpartx -dsv "$src_nbd"
    kpartx -dsv "$tmp_nbd"

    rm -f "$src_img_path" "$tmp_img_path"

    [[ -n "$msg" ]] && printf "\n%s" "$msg"

    return 0
}

trap 'cleanup "image creation failed"' 0

modprobe nbd

rm -f "$src_img_path" "$tmp_img_path"
echo "Cleanuping build env"
cleanup ""
if [ "${SE_BOOT}" = "1" ]; then
    echo "Finding host key files" 
    host_keys=""
    for i in $(ls ${HOST_KEYS_DIR}/*.crt); do
        echo "found host key file: \"${i}\""
        host_keys+="-k ${i} "
    done
    [[ -z $host_keys ]] && echo "Didn't find hosy key files, please set HOST_KEYS_DIR correctly "&& exit 1
fi

printf "\nCopying partitions from the base image $src_img_path\n"

qemu-img create -f qcow2 -b "$base_img_path" "$src_img_path" $disksize
qemu-img create -f qcow2 "$tmp_img_path" $disksize

qemu-nbd --connect="$src_nbd" "$src_img_path"
qemu-nbd --connect="$tmp_nbd" "$tmp_img_path"

declare -a parts

# https://alioth-lists.debian.net/pipermail/parted-devel/2006-December/000573.html
i=1
while IFS=':;' read -a part; do

    if (( $i == 1 )); then
        if [[ "${part[0]}" != BYT ]]; then
            echo "unrecognized parted output" 1>&2
            exit 1
        fi
    elif (( $i == 2 )); then

        if [[ "${part[0]}" != "$src_nbd" ]]; then
            echo "device path is not an nbd device" 1>&2
            exit 1
        fi
        if [[ "${part[3]}" != 512 || "${part[4]}" != 512 ]]; then
            echo "sector size is not 512 bytes" 1>&2
            exit 1
        fi

        disklabel=${part[5]}
        case "$disklabel" in
            msdos)
                ;;
            gpt)
                sgdisk -e "$src_nbd"
                ;;
            *)
                echo "unrecognized disk label: $disklabel" 1>&2
                exit 1
            ;;
        esac
    else
        part_number=${part[0]}
        part_offset=$(echo "${part[1]}" | sed -e 's/B$//')
        part_type=${part[4]}

        if [[ "$part_number" != 1 ]]; then
            parts+=("$part_number")
        else
            if [[ "$part_type" != ext4 ]]; then
                echo "fs type of partition 1 is not ext4" 1>&2
                exit 1
            fi
            target_offset=$(( $part_offset / 512 ))
            parted -s "$src_nbd" resizepart 1 100%
        fi
    fi

    (( i++ ))
done < <(parted -s -m "$src_nbd" unit B print)

sleep 1
resize2fs -f "${src_nbd}p1"

if [ "${SE_BOOT}" = "1" ]; then
    echo "Creating boot-se and root partitions" 
    parted -a optimal $tmp_nbd mklabel gpt \
        mkpart boot-se ext4 1MiB 256MiB \
        mkpart root 256MiB "${disksize}" \
        set 1 boot on
else
    case "$disklabel" in
        gpt)
            sgdisk "$src_nbd" -R "$tmp_nbd"
            sgdisk -G "$tmp_nbd"
            ;;
        msdos)
            sfdisk -d "$src_nbd" | sfdisk "$tmp_nbd"
            ;;
    esac
fi

if [ "${SE_BOOT}" = "1" ]; then
    echo "Waiting for the two nbd partitions to show up"
    while true; do
    sleep 1
    [ -e ${tmp_nbd}p2 ] && break
    done
    printf "\nFormatting boot-se partition\n"
    mke2fs -t ext4 -L boot-se ${tmp_nbd}p1
    export boot_uuid=$(blkid ${tmp_nbd}p1 -s PARTUUID -o value)
    printf "\nSetting up encrypted root partition\n"
    mkdir rootkeys || true
    mount -t tmpfs rootkeys ./rootkeys
    dd if=/dev/random of=./rootkeys/rootkey.bin bs=1 count=64 &> /dev/null
    echo YES | cryptsetup luksFormat --type luks2 ${tmp_nbd}p2 --key-file ./rootkeys/rootkey.bin
    echo "Setting LUKS name for root partition"
    LUKS_NAME="LUKS-$(blkid -s UUID -o value ${tmp_nbd}p2)"
    export LUKS_NAME
    echo "LUKS name is: $LUKS_NAME"
    cryptsetup open ${tmp_nbd}p2 $LUKS_NAME --key-file ./rootkeys/rootkey.bin
else
    for part_number in "${parts[@]}"; do
        dd if="${src_nbd}p$part_number" of="${tmp_nbd}p$part_number" bs=$((1024*1024))
    done
fi

printf "\nMounting the root partition\n"

src_part="${src_nbd}p1"
dst_part="${tmp_nbd}p1"

mkdir -p "$src_mnt"
mount "$src_part" "$src_mnt"

mount -t sysfs sysfs "$src_mnt/sys"
mount -t proc proc "$src_mnt/proc"
mount --bind /dev "$src_mnt/dev"
mount --bind /dev/pts "$src_mnt/dev/pts"

mount -t tmpfs tmpfs "$src_mnt/run"
mkdir -p "$src_mnt/run/systemd/resolve"
cp /run/systemd/resolve/resolv.conf "$src_mnt/run/systemd/resolve/resolv.conf"
cp /run/systemd/resolve/stub-resolv.conf "$src_mnt/run/systemd/resolve/stub-resolv.conf"

if (( ${#packages[@]} )); then
    printf "\nInstalling packages: ${packages[*]}\n"
    chroot "$src_mnt" apt-get update
    chroot "$src_mnt" apt-get install -y "${packages[@]}"
fi

case "$(uname -m)" in
    s390x)
        ;;
    *)
        printf "\nUpdating initramfs\n"
        echo -e "virtio_pci\nvirtio_blk" >> "$src_mnt/etc/initramfs-tools/modules"
        chroot "$src_mnt" update-initramfs  -u
        ;;

esac

chroot "$src_mnt" apt-get remove unattended-upgrades -y
chroot "$src_mnt" apt-get autoremove
chroot "$src_mnt" apt-get clean
chroot "$src_mnt" bash -c 'rm -rf /var/lib/apt/lists/*'

cp -a "$files_dir"/* "$src_mnt"

mkdir -p "$src_mnt/var/lib/kubelet"

umount "$src_mnt/run"
umount "$src_mnt/dev/pts"
umount "$src_mnt/dev"
umount "$src_mnt/proc"
umount "$src_mnt/sys"

mkdir -p "$dst_mnt"
if [ "${SE_BOOT}" = "1" ]; then
    dst_part="${tmp_nbd}p2"
    label="root"
    printf "\nFormatting root partition\n"
    mkfs.ext4 -L "$label" /dev/mapper/$LUKS_NAME
    mount /dev/mapper/$LUKS_NAME "$dst_mnt"
    mkdir ${dst_mnt}/etc
    mkdir ${dst_mnt}/boot-se
    mount -o norecovery ${tmp_nbd}p1 ${dst_mnt}/boot-se
else
    label=$(lsblk -n -o label "$src_part")
    mkfs.ext4 -L "$label" "$dst_part"
    mount "$dst_part" "$dst_mnt"
fi

echo "Copying the root filesystem"
tar_opts=(--numeric-owner --preserve-permissions --acl --selinux --xattrs --xattrs-include='*' --sparse)
tar -cf - "${tar_opts[@]}" --sort=none -C "$src_mnt" . | tar -xf - "${tar_opts[@]}" --preserve-order  -C "$dst_mnt"

echo "The root filesystem is ready"
sleep 1
umount "$src_mnt"
mount -t sysfs sysfs "$dst_mnt/sys"
mount -t proc proc "$dst_mnt/proc"
mount --bind /dev "$dst_mnt/dev"

if [ "${SE_BOOT}" = "1" ]; then
    printf "\nPreparing secure execution boot image\n"
    echo "mounting tmpfs to /etc/keys"
    mkdir -p ${dst_mnt}/etc/keys
    mount -t tmpfs keys ${dst_mnt}/etc/keys
    # ADD CONFIGURATION
    echo "adding fstab"
    cat <<END > ${dst_mnt}/etc/fstab
#This file was auto-generated
/dev/mapper/$LUKS_NAME    /        ext4  defaults 1 1
PARTUUID=$boot_uuid    /boot-se    ext4  norecovery 1 2
END
    echo "adding luks keyfile for fs"
    cp ./rootkeys/rootkey.bin ${dst_mnt}/etc/keys/luks-$(blkid -s UUID -o value /dev/mapper/$LUKS_NAME).key
    chmod 600 ${dst_mnt}/etc/keys/luks-$(blkid -s UUID -o value /dev/mapper/$LUKS_NAME).key
    cat <<END > ${dst_mnt}/etc/crypttab
#This file was auto-generated
$LUKS_NAME UUID=$(blkid -s UUID -o value ${dst_part}) /etc/keys/luks-$(blkid -s UUID -o value /dev/mapper/$LUKS_NAME).key luks,discard,initramfs
END
    chmod 744 ${dst_mnt}/etc/crypttab
    # Disable virtio_rng
    cat <<END > ${dst_mnt}/etc/modprobe.d/blacklist-virtio.conf
#don't trust rng from hypervisor
blacklist virtio_rng
END
    # Favor loading of TRNG module for newer Z machines
    echo 's390_trng' >> ${dst_mnt}/etc/modules

    # Prep files needed for mkinitrd (if running encrypted /)
    echo "KEYFILE_PATTERN=\"/etc/keys/*.key\"" >> ${dst_mnt}/etc/cryptsetup-initramfs/conf-hook
    echo "UMASK=0077" >> ${dst_mnt}/etc/initramfs-tools/initramfs.conf
    cat <<END > ${dst_mnt}/etc/zipl.conf
[defaultboot]
default=linux
target=/boot-se

targetbase=/dev/vda
targettype=scsi
targetblocksize=512
targetoffset=2048

[linux]
image = /boot-se/se.img
END
    echo "updating initial ram disk"
    chroot "$dst_mnt" update-initramfs -u || true
    echo "!!! Bootloader install errors prior to this line are intentional !!!!!" 1>&2
    printf "\nGenerating an IBM Secure Execution image\n"
    # Clean up kernel names and make sure they are where we expect them
    KERNEL_FILE=$(readlink ${dst_mnt}/boot/vmlinuz)
    echo "using kernel: ${KERNEL_FILE}"
    INITRD_FILE=$(readlink ${dst_mnt}/boot/initrd.img)
    echo "using initrd: ${INITRD_FILE}"
    SE_PARMLINE="root=/dev/mapper/$LUKS_NAME console=ttysclp0 quiet panic=0 rd.shell=0 blacklist=virtio_rng swiotlb=262144"
    echo "${SE_PARMLINE}" > ${dst_mnt}/boot/parmfile
    /usr/bin/genprotimg \
        -i ${dst_mnt}/boot/${KERNEL_FILE} \
        -r ${dst_mnt}/boot/${INITRD_FILE} \
        -p ${dst_mnt}/boot/parmfile \
        --no-verify \
        ${host_keys} \
        -o "$dst_mnt"/boot-se/se.img
    # exit and throw an error if no se image was created
    [ ! -e $dst_mnt/boot-se/se.img ] && exit 1
    # if building the image succeeded wipe /boot
    rm -rf ${dst_mnt}/boot/*
    printf "\nRunning zipl to prepare boot partition\n"
    chroot $dst_mnt zipl --targetbase $tmp_nbd \
    --targettype scsi \
    --targetblocksize 512 \
    --targetoffset 2048 \
    --target /boot-se \
    --image /boot-se/se.img
    printf "\nClean luks keyfile\n"
    umount ./rootkeys/ || true
    rm -rf ./rootkeys/
    umount $dst_mnt/etc/keys
    umount $dst_mnt/boot-se
else
    case "$(uname -m)" in
        s390x)
            printf "\nExecuting zipl"
            helper="$dst_mnt/lib/s390-tools/zipl_helper.nbd"
            cat <<END > "$helper"
echo "targetbase=$tmp_nbd"
echo "targettype=scsi"
echo "targetblocksize=512"
echo "targetoffset=$target_offset"
END
            chmod 755 "$helper"
            chroot "$dst_mnt" zipl -V
            rm "$helper"
            ;;
        *)
            printf "\nUpdating GRUB settings"
            sed -i -r -e 's|^GRUB_CMDLINE_LINUX=|#\0"|' "$dst_mnt/etc/default/grub"
            cat <<END >> "$dst_mnt/etc/default/grub"

GRUB_CMDLINE_LINUX="nomodeset nofb vga=normal console=ttyS0"
GRUB_DISABLE_LINUX_UUID=true
GRUB_DISABLE_OS_PROBER=true
GRUB_DEVICE="LABEL=cloudimg-rootfs"
END
            chroot "$dst_mnt" update-grub
            ;;
    esac
fi

umount "$dst_mnt/dev"
umount "$dst_mnt/proc"
umount "$dst_mnt/sys"
umount "$dst_mnt"

if [ "${SE_BOOT}" = "1" ]; then
    echo "Closing encrypted root partition"
    cryptsetup close $LUKS_NAME
fi

qemu-nbd --disconnect "$src_nbd"
qemu-nbd --disconnect "$tmp_nbd"

sleep 1

printf "\nGenerating QCOW2 image file\n"

qemu-img convert -O qcow2 -c "$tmp_img_path" "$dst_img_path"

trap "" 0
cleanup ""

printf "\nCompleted image creation"
printf "\n$dst_img_path\n"

exit 0
