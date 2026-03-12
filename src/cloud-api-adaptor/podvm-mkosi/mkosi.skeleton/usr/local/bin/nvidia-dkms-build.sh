#!/usr/bin/env bash
set -euxo pipefail

nv_src=$(ls -1d /usr/src/nvidia-* 2>/dev/null | head -1)
if [ -z "${nv_src}" ]; then
    echo "No NVIDIA source tree found in /usr/src/, skipping DKMS build"
    exit 0
fi

nv_ver=$(basename "${nv_src}" | sed 's/^nvidia-//')
kernel_ver=$(uname -r)

if ! dkms status "nvidia/${nv_ver}" 2>/dev/null | grep -q "${kernel_ver}.*installed"; then
    dkms add "nvidia/${nv_ver}" 2>/dev/null || true
    echo "Building NVIDIA ${nv_ver} modules for kernel ${kernel_ver}..."
    dkms autoinstall --kernelver "${kernel_ver}"
    depmod -a "${kernel_ver}"
    echo "NVIDIA DKMS build complete"
else
    echo "NVIDIA ${nv_ver} modules already installed for ${kernel_ver}"
fi

modprobe nvidia
modprobe nvidia-uvm
