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

# install required packages
if [ "$CLOUD_PROVIDER" == "vsphere" ]
then
# Add vsphere specific commands to execute on remote
    case $PODVM_DISTRO in
    centos)
        #fallthrough
        ;&
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

if [[ "$CLOUD_PROVIDER" == "azure" && "$PODVM_DISTRO" == "ubuntu" ]]; then
    export DEBIAN_FRONTEND=noninteractive
    sudo apt-get update
    sudo apt-get install -y libtss2-tctildr0
fi

# Setup oneshot systemd service for AWS and Azure to enable NAT rules
if [ "$CLOUD_PROVIDER" == "azure" ] || [ "$CLOUD_PROVIDER" == "aws" ]
then
# Create a oneshot systemd service to execute setup-nat-for-imds.sh
# during first boot
cat <<'END' > /etc/systemd/system/setup-nat-for-imds.service
[Unit]
Description=Setup NAT for IMDS
After=agent-protocol-forwarder.service
Wants=agent-protocol-forwarder.service
DefaultDependencies=no

[Service]
Type=oneshot
ExecStart=/usr/local/bin/setup-nat-for-imds.sh

[Install]
WantedBy=multi-user.target
END

# Add a link to the service in multi-user.target.wants
ln -s /etc/systemd/system/setup-nat-for-imds.service /etc/systemd/system/multi-user.target.wants/setup-nat-for-imds.service

# Create a script to setup NAT for IMDS
cat <<'END' > /usr/local/bin/setup-nat-for-imds.sh
#!/bin/bash
# This script sets up NAT for IMDS on Azure/AWS
# This is required for IMDS to work on Azure/AWS
# This script is executed as a oneshot systemd service
# during first boot

set -euo pipefail

IMDS_IP="169.254.169.254"
DUMMY_IP="169.254.99.99"

# trap errors
trap 'echo "Error: $0:$LINENO stopped"; exit 1' ERR INT

# Function to setup veth pair
function setup_proxy_arp() {
  local pod_ip
  pod_ip=$(ip netns exec podns ip route get "$IMDS_IP" | awk 'NR == 1 { print $7 }')

  ip link add veth2 type veth peer name veth1
  # Proxy arp does not get enabled when no IP address is assigned
  ip address add "$DUMMY_IP/32" dev veth1
  ip link set up dev veth1

  sysctl -w net.ipv4.ip_forward=1
  sysctl -w net.ipv4.conf.veth1.proxy_arp=1
  sysctl -w net.ipv4.neigh.veth1.proxy_delay=0

  ip link set veth2 netns podns
  ip netns exec podns ip link set up dev veth2
  ip netns exec podns ip route add "$IMDS_IP/32" dev veth2

  ip route add "$pod_ip/32" dev veth1

  local hwaddr
  hwaddr=$(ip netns exec podns ip -br link show veth2 | awk 'NR==1 { print $3 }')
  ip neigh replace "$pod_ip" dev veth1 lladdr "$hwaddr"

  iptables -t nat -A POSTROUTING -s "$pod_ip/32" -d "$IMDS_IP/32" -j MASQUERADE
}

# Execute functions
setup_proxy_arp
END

# Make the script executable
chmod +x /usr/local/bin/setup-nat-for-imds.sh

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

# Disable unnecessary systemd services

case $PODVM_DISTRO in
    centos)
        #fallthrough
        ;&
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

exit 0
