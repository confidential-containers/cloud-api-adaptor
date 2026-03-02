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
printf "\nCopying partitions from the base image $src_img_path\n"

cp -f "$base_img_path" "$src_img_path"
qemu-img resize "$src_img_path" $disksize
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
        part_type=${part[4]}

        if [[ "$part_number" != 1 ]]; then
            parts+=("$part_number")
        else
            if [[ "$part_type" != ext4 ]]; then
                echo "fs type of partition 1 is not ext4" 1>&2
                exit 1
            fi
            parted -s "$src_nbd" resizepart 1 100%
        fi
    fi

    (( i++ ))
done < <(parted -s -m "$src_nbd" unit B print)

sleep 1
resize2fs -f "${src_nbd}p1"

case "$disklabel" in
    gpt)
        sgdisk "$src_nbd" -R "$tmp_nbd"
        sgdisk -G "$tmp_nbd"
        ;;
    msdos)
        sfdisk -d "$src_nbd" | sfdisk "$tmp_nbd"
        ;;
esac

for part_number in "${parts[@]}"; do
    dd if="${src_nbd}p$part_number" of="${tmp_nbd}p$part_number" bs=$((1024*1024))
done

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

printf "\nUpdating initramfs\n"
echo -e "virtio_pci\nvirtio_blk" >> "$src_mnt/etc/initramfs-tools/modules"
chroot "$src_mnt" update-initramfs  -u

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
label=$(lsblk -n -o label "$src_part")
mkfs.ext4 -L "$label" "$dst_part"
mount "$dst_part" "$dst_mnt"

echo "Copying the root filesystem"
tar_opts=(--numeric-owner --preserve-permissions --acl --selinux --xattrs --xattrs-include='*' --sparse)
tar -cf - "${tar_opts[@]}" --sort=none -C "$src_mnt" . | tar -xf - "${tar_opts[@]}" --preserve-order  -C "$dst_mnt"

echo "The root filesystem is ready"
sleep 1
umount "$src_mnt"
mount -t sysfs sysfs "$dst_mnt/sys"
mount -t proc proc "$dst_mnt/proc"
mount --bind /dev "$dst_mnt/dev"

printf "\nUpdating GRUB settings"
sed -i -r -e 's|^GRUB_CMDLINE_LINUX=|#\0"|' "$dst_mnt/etc/default/grub"
cat <<END >> "$dst_mnt/etc/default/grub"

GRUB_CMDLINE_LINUX="nomodeset nofb vga=normal console=ttyS0"
GRUB_DISABLE_LINUX_UUID=true
GRUB_DISABLE_OS_PROBER=true
GRUB_DEVICE="LABEL=cloudimg-rootfs"
END
chroot "$dst_mnt" update-grub

umount "$dst_mnt/dev"
umount "$dst_mnt/proc"
umount "$dst_mnt/sys"
umount "$dst_mnt"

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
