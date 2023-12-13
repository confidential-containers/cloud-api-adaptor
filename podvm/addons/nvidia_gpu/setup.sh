#!/bin/bash

# Create the prestart hook directory
mkdir -p /usr/share/oci/hooks/prestart

# Add hook script
cat <<'END' >  /usr/share/oci/hooks/prestart/nvidia-container-toolkit.sh
#!/bin/bash -x

/usr/bin/nvidia-container-toolkit -debug "$@"
END

# Make the script executable
chmod +x /usr/share/oci/hooks/prestart/nvidia-container-toolkit.sh

# PODVM_DISTRO variable is set as part of the podvm image build process
# and available inside the packer VM
# Add NVIDIA packages
if  [[ "$PODVM_DISTRO" == "ubuntu" ]]; then
    export DEBIAN_FRONTEND=noninteractive
    distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
    curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
    curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
    apt-get -q update -y
    apt-get -q install -y nvidia-container-toolkit
    apt-get -q install -y wget build-essential pkg-config
    apt-get -q install -y nvidia-driver-530

    sed -i "s/#debug/debug/g"                                           /etc/nvidia-container-runtime/config.toml
    sed -i "s|/var/log|/var/log/nvidia-kata-container|g"                /etc/nvidia-container-runtime/config.toml
    sed -i "s/#no-cgroups = false/no-cgroups = true/g"                  /etc/nvidia-container-runtime/config.toml
    sed -i "/\[nvidia-container-cli\]/a no-pivot = true"                /etc/nvidia-container-runtime/config.toml
    sed -i "s/disable-require = false/disable-require = true/g"         /etc/nvidia-container-runtime/config.toml

    apt remove -q -y build-essential
fi
if  [[ "$PODVM_DISTRO" == "rhel" ]]; then
    dnf config-manager --add-repo http://developer.download.nvidia.com/compute/cuda/repos/rhel9/x86_64/cuda-rhel9.repo
    dnf install -q -y kernel-devel-"$(uname -r)" kernel-headers-"$(uname -r)"

    dnf install -q -y nvidia-container-toolkit
    dnf -q -y module install nvidia-driver:latest

    sed -i "s/#debug/debug/g"                                           /etc/nvidia-container-runtime/config.toml
    sed -i "s|/var/log|/var/log/nvidia-kata-container|g"                /etc/nvidia-container-runtime/config.toml
    sed -i "s/#no-cgroups = false/no-cgroups = true/g"                  /etc/nvidia-container-runtime/config.toml
    sed -i "/\[nvidia-container-cli\]/a no-pivot = true"                /etc/nvidia-container-runtime/config.toml
    sed -i "s/disable-require = false/disable-require = true/g"         /etc/nvidia-container-runtime/config.toml

fi
