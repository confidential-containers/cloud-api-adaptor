#!/bin/bash

# Function to remove SSH server on Ubuntu
remove_ssh_ubuntu() {
    echo "Detected Ubuntu. Removing openssh-server using apt-get."
    apt-get remove -y openssh-server
}

# Function to remove SSH server on RHEL
remove_ssh_rhel() {
    echo "Detected RHEL. Removing openssh-server using dnf."
    dnf remove -y openssh-server
}

# Detect the operating system
if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "$ID" in
        ubuntu)
            remove_ssh_ubuntu
            ;;
        rhel | centos | fedora)
            remove_ssh_rhel
            ;;
        *)
            echo "Unsupported OS: $ID"
            exit 1
            ;;
    esac
else
    echo "Cannot detect the operating system. /etc/os-release not found."
    exit 1
fi

