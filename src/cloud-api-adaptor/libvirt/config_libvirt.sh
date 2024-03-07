#!/bin/bash
#
# Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Install dependency packages for libvirt and kcli
#
set -o errexit
set -o nounset
set -o pipefail

ARCH=$(uname -m)
TARGET_ARCH=${ARCH/x86_64/amd64}

installGolang() {
    export PATH=/usr/local/go/bin:$PATH
    export GOROOT=/usr/local/go
    export GOPATH=$HOME/go
    if ! command -v "yq" >/dev/null; then
        echo "Installing latest yq"
        sudo wget https://github.com/mikefarah/yq/releases/latest/download/yq_linux_${TARGET_ARCH} -O /usr/bin/yq && sudo chmod a+x /usr/bin/yq
    fi
    REQUIRED_GO_VERSION="$(yq '.tools.golang' versions.yaml)"
    if [[ -d /usr/local/go ]]; then
        installed_go_version=$(v=$(go version | awk '{print $3}') && echo ${v#go})
        if [[ "$(printf '%s\n' "$REQUIRED_GO_VERSION" "$installed_go_version" | sort -V | head -1)" != "$REQUIRED_GO_VERSION" ]]; then
            echo "Warning: Found ${installed_go_version} at /usr/local/go, is lower than our required $REQUIRED_GO_VERSION"
            echo "Please run \"rm -rf /usr/local/go\" and run this script again."
            exit 1
        else
            echo "Found ${installed_go_version} at /usr/local/go, good to go"
        fi
    else
        wget -q "https://dl.google.com/go/go${REQUIRED_GO_VERSION}.linux-${TARGET_ARCH}.tar.gz"
        sudo tar -C /usr/local -xzf go${REQUIRED_GO_VERSION}.linux-${TARGET_ARCH}.tar.gz
        echo "Installed golang with ${REQUIRED_GO_VERSION}"
    fi
    mkdir -p $HOME/go
}

installLibvirt() {
    sudo DEBIAN_FRONTEND=noninteractive apt-get update -y > /dev/null
    sudo DEBIAN_FRONTEND=noninteractive apt-get install python3-pip genisoimage qemu-kvm libvirt-daemon-system libvirt-dev cpu-checker -y
    kvm-ok
    # Create the default storage pool if not defined.
    echo "Setup Libvirt default storage pool..."

    if ! sudo virsh pool-list --all | grep default >/dev/null; then
        sudo virsh pool-define-as default dir - - - - "/var/lib/libvirt/images"
        sudo virsh pool-build default
        sudo virsh pool-start default
        sudo setfacl -m "u:${USER}:rwx" /var/lib/libvirt/images
        sudo adduser "$USER" libvirt
        sudo setfacl -m "u:${USER}:rwx" /var/run/libvirt/libvirt-sock
    fi
}

installKcli() {
    if ! command -v kcli >/dev/null; then
        echo "Installing kcli"
        if [[ ${TARGET_ARCH} == "s390x" ]]; then
            # Installation of the kcli is supported exclusively for s390x machines using pypi
            sudo pip3 install kcli
        else
            curl https://raw.githubusercontent.com/karmab/kcli/main/install.sh | sudo bash
        fi
    fi
}

installK8sclis() {
    if ! command -v kubectl >/dev/null; then
        sudo curl -s "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/${TARGET_ARCH}/kubectl" \
            -o /usr/local/bin/kubectl
        sudo chmod a+x /usr/local/bin/kubectl
    fi
}

echo "Installing Go..."
installGolang
echo "Installing Libvirt..."
installLibvirt
echo "Installing kcli..."
installKcli
echo "Installing kubectl..."
installK8sclis

# kcli needs a pair of keys to setup the VMs
[ -f $HOME/.ssh/id_rsa ] || ssh-keygen -t rsa -f $HOME/.ssh/id_rsa -N ""

pushd install/overlays/libvirt
cp $HOME/.ssh/id_rsa* .
cat id_rsa.pub >> $HOME/.ssh/authorized_keys
chmod 600 $HOME/.ssh/authorized_keys

echo "Verifing libvirt connection..."
IP="$(hostname -I | cut -d' ' -f1)"
virsh -c "qemu+ssh://${USER}@${IP}/system?keyfile=$(pwd)/id_rsa&no_verify=1" nodeinfo
popd

rm -f libvirt.properties
echo "libvirt_uri=\"qemu+ssh://${USER}@${IP}/system?no_verify=1\"" >> libvirt.properties
echo "libvirt_ssh_key_file=\"id_rsa\"" >> libvirt.properties
