#!/usr/bin/env bash
set -euxo pipefail

# Wait for /dev/sda2 to appear
DEVICE="/dev/sda2"
MAX_WAIT=30
WAITED=0

while [ ! -b "$DEVICE" ] && [ $WAITED -lt $MAX_WAIT ]; do
    echo "Waiting for $DEVICE to appear... ($WAITED/$MAX_WAIT seconds)"
    sleep 1
    WAITED=$((WAITED + 1))
done

if [ ! -b "$DEVICE" ]; then
    echo "ERROR: $DEVICE did not appear within $MAX_WAIT seconds"
    exit 1
fi

echo "$DEVICE is available"

# Format as ext4 only if it has no filesystem
if ! blkid "$DEVICE" >/dev/null 2>&1; then
    echo "Formatting $DEVICE with ext4..."
    mkfs.ext4 -F "$DEVICE"
else
    echo "$DEVICE already has a filesystem: $(blkid -o value -s TYPE "$DEVICE")"
fi

# Mount point
MOUNT_POINT="/mnt/data"
mkdir -p "$MOUNT_POINT"

# Mount the device if not already mounted
if ! mountpoint -q "$MOUNT_POINT"; then
    mount "$DEVICE" "$MOUNT_POINT"
    echo "Mounted $DEVICE to $MOUNT_POINT"
else
    echo "$MOUNT_POINT is already mounted"
fi

# Create directories for bind mounts
mkdir -p "$MOUNT_POINT/kubelet"
mkdir -p "$MOUNT_POINT/containerd"

# Bind-mount /mnt/data/kubelet → /var/lib/kubelet
KUBELET_TARGET="/var/lib/kubelet"
if [ -d "$KUBELET_TARGET" ] && [ "$(ls -A "$KUBELET_TARGET" 2>/dev/null)" ]; then
    echo "Copying existing content from $KUBELET_TARGET to $MOUNT_POINT/kubelet..."
    rsync -aHAX "$KUBELET_TARGET"/ "$MOUNT_POINT/kubelet"/ 2>/dev/null || true
fi
mkdir -p "$KUBELET_TARGET"
if ! mountpoint -q "$KUBELET_TARGET"; then
    mount --bind "$MOUNT_POINT/kubelet" "$KUBELET_TARGET"
    echo "Bind-mounted $MOUNT_POINT/kubelet to $KUBELET_TARGET"
else
    echo "$KUBELET_TARGET is already mounted"
fi

# Bind-mount /mnt/data/containerd → /var/lib/containerd
CONTAINERD_TARGET="/var/lib/containerd"
if [ -d "$CONTAINERD_TARGET" ] && [ "$(ls -A "$CONTAINERD_TARGET" 2>/dev/null)" ]; then
    echo "Copying existing content from $CONTAINERD_TARGET to $MOUNT_POINT/containerd..."
    rsync -aHAX "$CONTAINERD_TARGET"/ "$MOUNT_POINT/containerd"/ 2>/dev/null || true
fi
mkdir -p "$CONTAINERD_TARGET"
if ! mountpoint -q "$CONTAINERD_TARGET"; then
    mount --bind "$MOUNT_POINT/containerd" "$CONTAINERD_TARGET"
    echo "Bind-mounted $MOUNT_POINT/containerd to $CONTAINERD_TARGET"
else
    echo "$CONTAINERD_TARGET is already mounted"
fi

echo "Data mount setup complete. Kubernetes storage is now on $DEVICE"
