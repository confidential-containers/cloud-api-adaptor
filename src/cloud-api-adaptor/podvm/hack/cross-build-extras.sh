#!/bin/bash
# cross-build-extras.sh
# Install the additional requires for cross-compilation
# of podvm image binaries

# If ARCH is not set, exit
[[ -z $ARCH ]] && exit 0

# Normalise ARCH (if input is amd64 use x86_64)
ARCH=${ARCH/amd64/x86_64}

# If ARCH is equal to HOST, exit
[[ $ARCH = $(uname -m) ]] && exit 0

# Only gnu is available for s390x
libc=$([[ $ARCH =~ s390x ]] && echo "gnu" || echo "musl")
rustTarget="$ARCH-unknown-linux-$libc"

rustup target add "$rustTarget"
apt install -y "qemu-system-$ARCH"
apt install -y "gcc-$ARCH-linux-$libc"
