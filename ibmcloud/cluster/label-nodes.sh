#!/bin/bash
# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

region="$1"
zone="$2"
subnet_id="$3"

nodes=$(kubectl --kubeconfig config get nodes -o name)

worker=
for node in $nodes; do
    if [ -n "$worker" ]; then
        kubectl --kubeconfig config label "$node" node-role.kubernetes.io/worker=
    fi
    worker=true
    kubectl --kubeconfig config label "$node" "topology.kubernetes.io/region=$region"
    kubectl --kubeconfig config label "$node" "topology.kubernetes.io/zone=$zone"
    kubectl --kubeconfig config label "$node" "ibm-cloud.kubernetes.io/subnet-id=$subnet_id"
done
