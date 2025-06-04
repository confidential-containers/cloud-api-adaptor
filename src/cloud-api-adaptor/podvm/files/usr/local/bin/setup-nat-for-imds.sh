#!/bin/bash
# This script sets up NAT for IMDS on Azure/AWS
# It can be safely re-executed to complete or fix a partial setup.

set -euo pipefail

IMDS_IP="169.254.169.254"
DUMMY_IP="169.254.99.99"
VETH_HOST="veth1"
VETH_NS="veth2"
NETNS="podns"

# trap errors
trap 'echo "Error: $0:$LINENO stopped"; exit 1' ERR INT

# Function to check if a network device exists
function link_exists() {
  ip link show "$1" &>/dev/null
}

# Function to setup veth pair and routing/NAT
function setup_proxy_arp() {
  local pod_ip
  pod_ip=$(ip netns exec "$NETNS" ip route get "$IMDS_IP" | awk '{ for(i=1; i<=NF; i++) { if($i == "src") { print $(i+1); break; } } }')

  # Create veth pair only if it doesn't exist
  if ! link_exists "$VETH_HOST"; then
    ip link add "$VETH_NS" type veth peer name "$VETH_HOST"
  fi

  # Assign IP to veth1 if not already assigned
  if ! ip address show dev "$VETH_HOST" | grep -q "$DUMMY_IP"; then
    ip address add "$DUMMY_IP/32" dev "$VETH_HOST"
  fi

  ip link set up dev "$VETH_HOST"

  sysctl -w net.ipv4.ip_forward=1 >/dev/null
  sysctl -w net.ipv4.conf."$VETH_HOST".proxy_arp=1 >/dev/null
  sysctl -w net.ipv4.neigh."$VETH_HOST".proxy_delay=0 >/dev/null

  # Move veth2 into the pod namespace if not already
  if ! ip netns exec "$NETNS" ip link show "$VETH_NS" &>/dev/null; then
    ip link set "$VETH_NS" netns "$NETNS"
  fi

  ip netns exec "$NETNS" ip link set up dev "$VETH_NS"

  # Add route to IMDS via veth2 if not present
  if ! ip netns exec "$NETNS" ip route show | grep -q "$IMDS_IP"; then
    ip netns exec "$NETNS" ip route add "$IMDS_IP/32" dev "$VETH_NS"
  fi

  # Route pod IP via veth1
  if ! ip route get "$pod_ip" | grep -q "dev $VETH_HOST"; then
    ip route add "$pod_ip/32" dev "$VETH_HOST"
  fi

  # Set static ARP entry
  local hwaddr
  hwaddr=$(ip netns exec "$NETNS" ip -br link show "$VETH_NS" | awk 'NR==1 { print $3 }')
  ip neigh replace "$pod_ip" dev "$VETH_HOST" lladdr "$hwaddr"

  # Add iptables NAT rule only if not present
  if ! iptables -t nat -C POSTROUTING -s "$pod_ip/32" -d "$IMDS_IP/32" -j MASQUERADE 2>/dev/null; then
    iptables -t nat -A POSTROUTING -s "$pod_ip/32" -d "$IMDS_IP/32" -j MASQUERADE
  fi
}

# Execute functions
setup_proxy_arp
