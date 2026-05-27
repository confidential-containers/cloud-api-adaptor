#!/bin/bash
# cross-build-extras.sh
# Install the additional requires for cross-compilation
# of podvm image binaries

# If ARCH is not set, exit
[[ -z $ARCH ]] && exit 0

# If ARCH is equal to HOST, exit (no cross-compilation needed)
# Use replacement pattern to normalize arm64 to aarch64 for comparison
[[ "${ARCH/arm64/aarch64}" = "$(uname -m)" ]] && exit 0

# Only gnu is available for s390x and aarch64
libc=$([[ $ARCH =~ s390x || $ARCH =~ aarch64 ]] && echo "gnu" || echo "musl")
rustTarget="$ARCH-unknown-linux-$libc"

rustup target add "$rustTarget"

source /etc/os-release || source /usr/lib/os-release
if [[ ${ID_LIKE:-} == *"debian"* ]]; then
    apt install -y "qemu-system-$ARCH"
    apt install -y "gcc-$ARCH-linux-$libc"
elif [[ "${ID_LIKE:-}" =~ "fedora" ]] || [[ "${ID:-}" =~ "fedora" ]]; then
    dnf install -y "qemu-system-$ARCH"
    dnf install -y "gcc-$ARCH-linux-$libc"
else
    echo "Unsupported distro $ID"; exit 1
fi
