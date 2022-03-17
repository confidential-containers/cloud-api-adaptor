#!/bin/bash
#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit -o pipefail -o nounset

cd "$(dirname "${BASH_SOURCE[0]}")"

function usage() {
    echo "Usage: $0 --bastion-node <bastion host> --control-plane-node --worker-nodes <worker hosts> [--ssh-private-key <ssh private key file>]"
}

declare -a workers

while (( $# )); do
    case "$1" in
        --control-plane) control_plane=$2 ;;
        --workers)       IFS=', ' read -a workers <<< "$2" ;;
        --bastion)       bastion=$2 ;;
        --ssh-private-key)    ssh_private_key=$2 ;;
        --help)     usage; exit 0 ;;
        *)          usage 1>&2; exit 1;;
    esac
    shift 2
done

if [[ -z "${control_plane-}" || "${#workers[@]}" -eq 0 || -z "${bastion-}" ]]; then
    usage 1>&2
    exit 1
fi

tmpdir=$(mktemp -d)
ssh_ctl_sock="$tmpdir/ssh-ctl.sock"
ssh_known_hosts="$tmpdir/known_hosts"

opts=(-l root -o StrictHostKeyChecking=accept-new -o "UserKnownHostsFile=$ssh_known_hosts" -o ProxyCommand="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -W %h:%p root@$bastion")

ssh "${opts[@]}" "root@$control_plane" \
    bash -x -c "true
if ! [[ -e /var/lib/kubelet/kubeadm-flags.env ]]; then
    kubeadm init --apiserver-advertise-address=$control_plane --pod-network-cidr=172.20.0.0/16 --cri-socket unix:///run/containerd/containerd.sock
    mkdir -p /root/.kube
    cp -f /etc/kubernetes/admin.conf /root/.kube/config

    # The default setting of Flannel conflicts with the IP range of the Tokyo zones
    # https://cloud.ibm.com/docs/vpc?topic=vpc-configuring-address-prefixes&locale=en

    curl -sL -o /tmp/kube-flannel.yml https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
    sed -i 's|"10.244.0.0/16"|"172.20.0.0/16"|' /tmp/kube-flannel.yml
    kubectl apply -f /tmp/kube-flannel.yml
fi
"

for worker in "${workers[@]}"; do
    ssh "${opts[@]}" "root@$worker" \
        bash -x -c "true
if ! [[ -e /var/lib/kubelet/kubeadm-flags.env ]]; then
    ssh -o StrictHostKeyChecking=accept-new root@$control_plane tar -cf - -C /root .kube/config | tar xf - -C /root
    \$(kubeadm token create --print-join-command) --cri-socket unix:///run/containerd/containerd.sock
fi
"
done
