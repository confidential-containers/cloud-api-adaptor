#!/usr/bin/env bash
#
# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Primarily used on Github workflows to debug failed pipelines.
#
# NOTE: if you want a debugger for MY_PROVIDER provider then you just need
# to create the debug_MY_PROVIDER function. Nothing else is needed.
#
# Not setting errexit, nounset, and pipefail because it is fine and should
# continue if any command fail.

CLOUD_PROVIDER=${CLOUD_PROVIDER:-}

# Get common debug information.
#
debug_common() {
    echo "::group::Kubernetes"
    kubectl get pods -A
    echo "::endgroup::"

    echo "::group::KBS installation"
    kubectl get pods -n coco-tenant
    kubectl describe pods -n coco-tenant
    echo "::endgroup::"

    echo "::group::CoCo and Peer Pods installation"
    kubectl get pods -n confidential-containers-system
    kubectl describe pods -n confidential-containers-system
    echo "::endgroup::"

    echo "::group::webhook installation logs"
    kubectl get pods -n peer-pods-webhook-system
    kubectl describe pods -n peer-pods-webhook-system
    echo "::endgroup::"

    echo "::group::cloud-api-adaptor logs"
    kubectl logs -l app=cloud-api-adaptor --tail=-1 -n confidential-containers-system
    echo "::endgroup::"

    echo "::group::kbs logs"
    kubectl logs deployment/kbs -n coco-tenant
    echo "::endgroup::"

    for ns in $(kubectl get ns -o name 2>/dev/null | sed 's#namespace/##' | grep "^coco-pp-"); do
        for pod in $(kubectl get pods -o name -n "$ns" 2>/dev/null); do
            echo "::group::Describe $pod (namespace/$ns)"
            kubectl describe "$pod" -n "$ns"
            echo "::endgroup::"
            echo "::group::Annotations $pod (namespace/$ns)"
            kubectl get "$pod" -n "$ns" -o jsonpath='{.metadata.annotations}'
            echo "::endgroup::"
        done
    done

    for worker in $(kubectl get node -o name -l node.kubernetes.io/worker 2>/dev/null); do
        echo "::group::journalctl -t kata ($worker)"
        kubectl debug --profile=general --image quay.io/prometheus/busybox -q -i \
            "$worker" -- chroot /host journalctl -x -t kata --no-pager
        echo "::endgroup::"
    done
}

# Debugger for Libvirt.
#
debug_libvirt() {
    echo "::group::Libvirt domains"
    sudo virsh list
    echo "::endgroup::"

    for podvm in $(sudo virsh list --name | grep "podvm-"); do
        echo "::group::podvm $podvm"
        sudo virsh dominfo "$podvm"
        sudo virsh domifaddr "$podvm"
        echo "::endgroup::"
    done

    echo "::group::podvm base volume"
    sudo virsh vol-info --pool default podvm-base.qcow2
    ls -lh /var/lib/libvirt/images/podvm-base.qcow2
    echo "::endgroup::"

    echo "::group::Check podvm base volume integrity"
    sudo qemu-img check /var/lib/libvirt/images/podvm-base.qcow2
    echo "::endgroup::"
}

main() {
    debug_common

    if [ -n "$CLOUD_PROVIDER" ]; then
        if ! type -a "debug_${CLOUD_PROVIDER}" &>/dev/null; then
            echo "INFO: Cannot get further information as debugger for ${CLOUD_PROVIDER} is not implemented"
        else
            "debug_${CLOUD_PROVIDER}"
        fi
    fi
}

main "$@"
