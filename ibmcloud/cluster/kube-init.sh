#!/bin/bash
#
# Copyright (c) 2023 IBM
#
# SPDX-License-Identifier: Apache-2.0
#

error() {
    echo "$1"
    exit 1
}

inventory=./ansible/inventory
ssh_options="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
[ -f "$inventory" ] || error "$inventory: file does not exist"

ips=()
start="[cluster]"
end=""
capture="false"
while read line; do
    if [ "$line" = "$end" ]; then
        capture="false"
    fi
    if [ $capture = "true" ]; then
        ips+=($line)
    fi
    if [ "$line" = "$start" ]; then
        capture="true"
    fi
done <$inventory

control_plane_lan_ip="$1"
control_plane_wan_ip="${ips[0]}"
control_plane_execute(){
    ssh $ssh_options "root@$control_plane_wan_ip" "$1"
}
control_plane_execute "kubeadm init --apiserver-advertise-address=$control_plane_lan_ip --apiserver-cert-extra-sans=$control_plane_wan_ip --pod-network-cidr=172.20.0.0/16"
control_plane_execute "mkdir -p /root/.kube"
control_plane_execute "cp -f /etc/kubernetes/admin.conf /root/.kube/config"
control_plane_execute "curl -sL -o /tmp/kube-flannel.yml https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml"
control_plane_execute "sed -i 's|\"10.244.0.0/16\"|\"172.20.0.0/16\"|' /tmp/kube-flannel.yml"
control_plane_execute "kubectl apply -f /tmp/kube-flannel.yml"

join_command=$(ssh $ssh_options "root@$control_plane_wan_ip" "/bin/bash -c 'kubeadm token create --print-join-command'")

for ip in "${ips[@]:1}"; do
    ssh $ssh_options "root@$ip" "/bin/bash -c '$join_command'"
done

scp $ssh_options root@$control_plane_wan_ip:/root/.kube/config config
sed -E -i "s|(\s+server: https://).*(:[0-9]+)|\1$control_plane_wan_ip\2|" config
