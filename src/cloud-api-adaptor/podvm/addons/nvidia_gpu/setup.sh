#!/bin/bash

NVIDIA_DRIVER_VERSION=${NVIDIA_DRIVER_VERSION:-535}
NVIDIA_USERSPACE_VERSION=${NVIDIA_USERSPACE_VERSION:-1.13.5-1}

NVIDIA_USERSPACE_PKGS=(nvidia-container-toolkit libnvidia-container1 libnvidia-container-tools)

# Create the prestart hook directory
mkdir -p /usr/share/oci/hooks/prestart

# Add hook script
cat <<'END' >  /usr/share/oci/hooks/prestart/nvidia-container-toolkit.sh
#!/bin/bash -x

# Log the o/p of the hook to a file
/usr/bin/nvidia-container-toolkit -debug "$@" > /var/log/nvidia-hook.log 2>&1
END

# Make the script executable
chmod +x /usr/share/oci/hooks/prestart/nvidia-container-toolkit.sh


# according to https://developer.download.nvidia.com/compute/cuda/repos/rhel9/x86_64/precompiled/
# the correct way to install pre-compiled drivers is to update kernel and opt-into a precompiled modularity stream,
# however, it may take some time between latest kernel version is released until the precompiled pkgs are available;
# which may cause driver failure.
# This function tries to update the kernel to the latest version supported by the nvidia precompiled drivers, if fails
# it upgrades to the latest kernel version available.
rhel_kernel_version_matching() {
    dnf -q -y module enable nvidia-driver:${NVIDIA_DRIVER_VERSION}
    local latest_nv_version # latest nvidia driver available version
    local latest_kmod_pkg_name # latest version of kmod-nvidia pkg name
    local matching_kernel_version # extracted matching kernel version from kmod-nvidia name
    latest_nv_version=$(repoquery --latest-limit 1 nvidia-driver  --queryformat "%{version}" -q)
    latest_kmod_pkg_name=$(repoquery --latest-limit 1 kmod-nvidia-${latest_nv_version}-* --queryformat "%{name}" -q | sort -V | tail -n 1)
    matching_kernel_version=$(echo ${latest_kmod_pkg_name} | sed "s/kmod-nvidia-${latest_nv_version}-//")
    echo "latest_nv_version: ${latest_nv_version}, latest_kmod_pkg_name: ${latest_kmod_pkg_name}, matching_kernel_version: ${matching_kernel_version}"
    dnf -q -y install kernel-${matching_kernel_version}*  kernel-core-${matching_kernel_version}*  kernel-modules-core-${matching_kernel_version}*  kernel-modules-${matching_kernel_version}*
    if [ $? -eq 0 ]; then
        return 0
    else
        dnf -q -y update kernel kernel-core kernel-modules-core kernel-modules
    fi
}

# PODVM_DISTRO variable is set as part of the podvm image build process
# and available inside the packer VM
# Add NVIDIA packages
if  [[ "$PODVM_DISTRO" == "ubuntu" ]]; then
    export DEBIAN_FRONTEND=noninteractive
    distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
    curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
    curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
    apt-get -q update -y
    apt-get -q install -y "${NVIDIA_USERSPACE_PKGS[@]/%/-${NVIDIA_USERSPACE_VERSION}}"
    apt-get -q install -y nvidia-driver-${NVIDIA_DRIVER_VERSION}
fi
if  [[ "$PODVM_DISTRO" == "rhel" ]]; then
    dnf config-manager --add-repo http://developer.download.nvidia.com/compute/cuda/repos/rhel9/x86_64/cuda-rhel9.repo
    rhel_kernel_version_matching

    dnf install -q -y "${NVIDIA_USERSPACE_PKGS[@]/%/-${NVIDIA_USERSPACE_VERSION}}"
    # This will use the default stream
    dnf -q -y module install nvidia-driver:${NVIDIA_DRIVER_VERSION}
fi

# Configure the settings for nvidia-container-runtime
sed -i "s/#debug/debug/g"                                           /etc/nvidia-container-runtime/config.toml
sed -i "s|/var/log|/var/log/nvidia-kata-container|g"                /etc/nvidia-container-runtime/config.toml
sed -i "s/#no-cgroups = false/no-cgroups = true/g"                  /etc/nvidia-container-runtime/config.toml
sed -i "/\[nvidia-container-cli\]/a no-pivot = true"                /etc/nvidia-container-runtime/config.toml
sed -i "s/disable-require = false/disable-require = true/g"         /etc/nvidia-container-runtime/config.toml
sed -i "s/info/debug/g"                                             /etc/nvidia-container-runtime/config.toml


