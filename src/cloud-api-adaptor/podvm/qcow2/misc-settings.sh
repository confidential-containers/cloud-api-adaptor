#!/bin/bash
# Uncomment the awk statement if you want to use mac as
# dhcp-identifier in Ubuntu
#awk '/^[[:space:]-]*dhcp4/{ split($0,arr,/dhcp4.*/)
#                           gsub(/-/," ", arr[1])
#                           rep=arr[1]
#                           print $0}
#                      rep{ printf "%s%s\n", rep, "dhcp-identifier: mac"
#                      rep=""
#                      next} 1' /etc/netplan/50-cloud-init.yaml | sudo tee /etc/netplan/50-cloud-init.yaml

# This ensures machine-id is generated during first boot and a unique
# dhcp IP is assigned to the VM
echo -n | sudo tee /etc/machine-id
#Lock password for the ssh user (peerpod) to disallow logins
sudo passwd -l peerpod

# Subscribe RHEL incase of ACTIVATION_KEY & ORG_ID provided.
if [[ -n "${ACTIVATION_KEY}" && -n "${ORG_ID}" ]]; then \
    subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}"
fi

# install required packages
if [ "$CLOUD_PROVIDER" == "vsphere" ]
then
# Add vsphere specific commands to execute on remote
    case $PODVM_DISTRO in
    rhel)
        (! dnf list --installed | grep open-vm-tools > /dev/null 2>&1) && \
        (! dnf -y install open-vm-tools) && \
             echo "$PODVM_DISTRO: Error installing package required for cloud provider: $CLOUD_PROVIDER" 1>&2 && exit 1
        ;;
    ubuntu)
        (! dpkg -l | grep open-vm-tools > /dev/null 2>&1) && apt-get update && \
        (! apt-get -y install open-vm-tools > /dev/null 2>&1) && \
             echo "$PODVM_DISTRO: Error installing package required for cloud provider: $CLOUD_PROVIDER" 1>&2  && exit 1
        ;;
    *)
        ;;
    esac
fi

if [[ "$CLOUD_PROVIDER" == "azure" || "$CLOUD_PROVIDER" == "generic" ]] && [[ "$PODVM_DISTRO" == "ubuntu" ]] && [[ "$ARCH" == "x86_64" ]]; then
    export DEBIAN_FRONTEND=noninteractive
    sudo apt-get update
    sudo apt-get install -y --no-install-recommends libtss2-tctildr0
fi

# Install iptables for all providers except docker/vsphere.
if [ "$CLOUD_PROVIDER" != "vsphere" ] && [ "$CLOUD_PROVIDER" != "docker" ]
then
    if [ ! -x "$(command -v iptables)" ]; then
        case $PODVM_DISTRO in
        rhel)
            dnf -q install iptables -y
            ;;
        ubuntu)
            apt-get -qq update && apt-get -qq install iptables -y
            ;;
        *)
            echo "\"iptables\" is missing and cannot be installed, setup-nat-for-imds.sh is likely to fail 1>&2"
            ;;
        esac
    fi
fi

if [ -e /etc/certificates/tls.crt ] && [ -e /etc/certificates/tls.key ] && [ -e /etc/certificates/ca.crt ]; then
    # Update systemd service file to add additional options
    cat <<END >> /etc/default/agent-protocol-forwarder
TLS_OPTIONS=-cert-file /etc/certificates/tls.crt -cert-key /etc/certificates/tls.key -ca-cert-file /etc/certificates/ca.crt
END
elif [ -e /etc/certificates/tls.crt ] && [ -e /etc/certificates/tls.key ] && [ ! -e /etc/certificates/ca.crt ]; then
    # Update systemd service file to add additional options
    cat <<END >> /etc/default/agent-protocol-forwarder
TLS_OPTIONS=-cert-file /etc/certificates/tls.crt -cert-key /etc/certificates/tls.key
END
fi

if [ -n "${FORWARDER_PORT}" ]; then
    cat <<END >> /etc/default/agent-protocol-forwarder
OPTIONS=-listen 0.0.0.0:${FORWARDER_PORT}
END
fi

# Disable unnecessary systemd services

case $PODVM_DISTRO in
    rhel)
        systemctl disable kdump.service
        systemctl disable tuned.service
        systemctl disable firewalld.service
        ;;
    ubuntu)
        systemctl disable apt-daily.service
        systemctl disable apt-daily.timer
        systemctl disable apt-daily-upgrade.timer
        systemctl disable apt-daily-upgrade.service
        systemctl disable snapd.service
        systemctl disable snapd.seeded.service
        systemctl disable snap.lxd.activate.service
        ;;
esac

# Unsubscribe RHEL incase of ACTIVATION_KEY & ORG_ID provided.
if [[ -n "${ACTIVATION_KEY}" && -n "${ORG_ID}" ]]; then \
    subscription-manager unregister
fi

exit 0
