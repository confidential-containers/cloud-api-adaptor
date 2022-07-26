#!/bin/bash

: "${LIBVIRT_POOL:=default}" "${LIBVIRT_NET:=default}"

mkdir -p /opt/data-dir
/opt/confidential-containers/bin/cloud-api-adaptor-libvirt libvirt \
    -uri ${LIBVIRT_URI}  \
    -data-dir /opt/data-dir \
    -pods-dir /run/peerpod/pods \
    -network-name ${LIBVIRT_NET} \
    -pool-name ${LIBVIRT_POOL} \
    -socket /run/peerpod/hypervisor.sock 
