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

    echo "::group::KBS installation - pod summary"
    kubectl get pods -n coco-trustee -o wide
    echo "::endgroup::"

    echo "::group::KBS installation - bootstrap jobs"
    kubectl get jobs -n coco-trustee
    kubectl describe jobs -n coco-trustee
    echo "::endgroup::"

    echo "::group::KBS installation - bootstrap job logs"
    for job_pod in $(kubectl get pods -n coco-trustee -o name 2>/dev/null | grep -E 'bootstrap|hook'); do
        echo "=== $job_pod ==="
        kubectl logs "$job_pod" -n coco-trustee --all-containers 2>/dev/null || true
        kubectl logs "$job_pod" -n coco-trustee --all-containers --previous 2>/dev/null || true
    done
    echo "::endgroup::"

    echo "::group::KBS installation - secrets"
    kubectl get secrets -n coco-trustee
    # List keys present in bootstrap secret without exposing values
    kubectl get secret trustee-bootstrap-user-keys -n coco-trustee \
        -o jsonpath='{range .data}{.key}{"\n"}{end}' 2>/dev/null || \
    kubectl get secret trustee-bootstrap-user-keys -n coco-trustee \
        -o go-template='{{range $k,$v := .data}}{{$k}}{{"\n"}}{{end}}' 2>/dev/null || true
    echo "::endgroup::"

    echo "::group::KBS installation - configmaps"
    kubectl get configmaps -n coco-trustee
    kubectl get configmap trustee-as-config -n coco-trustee -o yaml 2>/dev/null || true
    kubectl get configmap trustee-kbs-config -n coco-trustee -o yaml 2>/dev/null || true
    echo "::endgroup::"

    echo "::group::KBS installation - events"
    kubectl get events -n coco-trustee --sort-by='.lastTimestamp'
    echo "::endgroup::"

    echo "::group::KBS installation - describe pods"
    kubectl describe pods -n coco-trustee
    echo "::endgroup::"

    echo "::group::CoCo and Peer Pods installation"
    kubectl get pods -n confidential-containers-system
    kubectl describe pods -n confidential-containers-system
    echo "::endgroup::"

    echo "::group::kata-deploy logs"
    kubectl logs -l name=kata-deploy --tail=-1 -n confidential-containers-system
    echo "::endgroup::"

    echo "::group::webhook installation logs"
    kubectl get pods -n peer-pods-webhook-system
    kubectl describe pods -n peer-pods-webhook-system
    echo "::endgroup::"

    echo "::group::peerpodctrl installation logs"
    pod=$(kubectl get pod -o name -n confidential-containers-system | grep peerpodctrl-controller-manager)
    if [ -z "$pod" ]; then
        pod=$(kubectl get pod -o name -n confidential-containers-system | grep peerpod-ctrl-controller-manager)
    fi
    [ -n "$pod" ] && kubectl logs "$pod" --tail=-1 -n confidential-containers-system
    echo "::endgroup::"

    echo "::group::cloud-api-adaptor logs"
    kubectl logs -l app=cloud-api-adaptor --tail=-1 -n confidential-containers-system
    echo "::endgroup::"

    echo "::group::kbs logs"
    kubectl logs -l app=kbs --tail=-1 -n coco-trustee --all-containers
    kubectl logs -l app=kbs --tail=-1 -n coco-trustee --all-containers --previous 2>/dev/null || true
    echo "::endgroup::"

    echo "::group::attestation-service logs"
    kubectl logs -l app=attestation-service --tail=-1 -n coco-trustee --all-containers
    kubectl logs -l app=attestation-service --tail=-1 -n coco-trustee --all-containers --previous 2>/dev/null || true
    echo "::endgroup::"

    echo "::group::rvps logs"
    kubectl logs -l app=reference-value-provider-service --tail=-1 -n coco-trustee --all-containers
    kubectl logs -l app=reference-value-provider-service --tail=-1 -n coco-trustee --all-containers --previous 2>/dev/null || true
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
