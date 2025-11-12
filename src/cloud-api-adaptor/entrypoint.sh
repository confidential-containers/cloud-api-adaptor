#!/bin/bash

################################################################################
# IMPORTANT: Keep this entrypoint.sh minimal!
#
# Environment variables are now handled directly in Go via FlagRegistrar.
# Each cloud provider's manager.go ParseCmd() automatically converts env vars
# to flags - no shell scripting needed.
#
# Only add env-to-arg conversions here for exceptional cases that cannot be
# handled in Go code. For everything else, add env support in the provider's
# manager.go instead.
#
# If you find a specific case or pattern not covered by FlagRegistrar, consider
# implementing it in Go code so all providers can benefit from it.
################################################################################

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER}
# Enabling dynamically loaded cloud provider external plugin feature, disabled by default
ENABLE_CLOUD_PROVIDER_EXTERNAL_PLUGIN=${ENABLE_CLOUD_PROVIDER_EXTERNAL_PLUGIN:-false}

CRI_RUNTIME_ENDPOINT=${CRI_RUNTIME_ENDPOINT:-/run/cri-runtime.sock}
REMOTE_HYPERVISOR_ENDPOINT=${REMOTE_HYPERVISOR_ENDPOINT:-/run/peerpod/hypervisor.sock}
PEER_PODS_DIR=${PODS_DIR:-/run/peerpod/pods}

optionals+=""

# Ensure you add a space before the closing quote (") when updating the optionals
# example:
# following is the correct method: optionals+="-option val "
# following is the incorrect method: optionals+="-option val"

[[ "${PAUSE_IMAGE}" ]] && optionals+="-pause-image ${PAUSE_IMAGE} "
[[ "${TUNNEL_TYPE}" ]] && optionals+="-tunnel-type ${TUNNEL_TYPE} "
[[ "${VXLAN_PORT}" ]] && optionals+="-vxlan-port ${VXLAN_PORT} "
[[ "${CACERT_FILE}" ]] && optionals+="-ca-cert-file ${CACERT_FILE} "
[[ "${CERT_FILE}" ]] && [[ "${CERT_KEY}" ]] && optionals+="-cert-file ${CERT_FILE} -cert-key ${CERT_KEY} "
[[ "${TLS_SKIP_VERIFY}" ]] && optionals+="-tls-skip-verify "
[[ "${PROXY_TIMEOUT}" ]] && optionals+="-proxy-timeout ${PROXY_TIMEOUT} "
[[ "${INITDATA}" ]] && optionals+="-initdata ${INITDATA} "
[[ "${FORWARDER_PORT}" ]] && optionals+="-forwarder-port ${FORWARDER_PORT} "
[[ "${CLOUD_CONFIG_VERIFY}" == "true" ]] && optionals+="-cloud-config-verify "
[[ "${SECURE_COMMS}" == "true" ]] && optionals+="-secure-comms "
[[ "${SECURE_COMMS_NO_TRUSTEE}" == "true" ]] && optionals+="-secure-comms-no-trustee "
[[ "${SECURE_COMMS_INBOUNDS}" ]] && optionals+="-secure-comms-inbounds ${SECURE_COMMS_INBOUNDS} "
[[ "${SECURE_COMMS_OUTBOUNDS}" ]] && optionals+="-secure-comms-outbounds ${SECURE_COMMS_OUTBOUNDS} "
[[ "${SECURE_COMMS_PP_INBOUNDS}" ]] && optionals+="-secure-comms-pp-inbounds ${SECURE_COMMS_PP_INBOUNDS} "
[[ "${SECURE_COMMS_PP_OUTBOUNDS}" ]] && optionals+="-secure-comms-pp-outbounds ${SECURE_COMMS_PP_OUTBOUNDS} "
[[ "${SECURE_COMMS_KBS_ADDR}" ]] && optionals+="-secure-comms-kbs ${SECURE_COMMS_KBS_ADDR} "
[[ "${PEERPODS_LIMIT_PER_NODE}" ]] && optionals+="-peerpods-limit-per-node ${PEERPODS_LIMIT_PER_NODE} "
[[ "${DISABLECVM}" == "true" ]] && optionals+="-disable-cvm "
[[ "${ENABLE_SCRATCH_SPACE}" == "true" ]] && optionals+="-enable-scratch-space "

test_vars() {
    for i in "$@"; do
        [ -z "${!i}" ] && echo "\$$i is NOT set" && EXT=1
    done
    [[ -n $EXT ]] && exit 1
}

one_of() {
    for i in "$@"; do
        [ -n "${!i}" ] && echo "\$$i is SET" && EXIST=1
    done
    [[ -z $EXIST ]] && echo "At least one of these must be SET: $*" && exit 1
}

aws() {
    test_vars AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

    # Global flags without env var support - still need conversion
    [[ "${EXTERNAL_NETWORK_VIA_PODVM}" ]] && optionals+="-ext-network-via-podvm  "
    [[ "${POD_SUBNET_CIDRS}" ]] && optionals+="-pod-subnet-cidrs ${POD_SUBNET_CIDRS} "

    set -x
    exec cloud-api-adaptor aws \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}

}

azure() {
    test_vars AZURE_CLIENT_ID AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID AZURE_RESOURCE_GROUP AZURE_SUBNET_ID AZURE_IMAGE_ID

    set -x
    exec cloud-api-adaptor azure \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}
}

alibabacloud() {
    one_of ALIBABACLOUD_ACCESS_KEY_ID ALIBABA_CLOUD_ROLE_ARN

    # TODO: Variable name mismatch - kustomization/entrypoint uses INSTANCE_TYPE
    # but manager.go expects PODVM_INSTANCE_TYPE. Consider standardizing in future.
    [[ "${INSTANCE_TYPE}" ]] && optionals+=" -instance-type ${INSTANCE_TYPE} "
    # Global flag without env var support - still need conversion
    [[ "${EXTERNAL_NETWORK_VIA_PODVM}" ]] && optionals+=" -ext-network-via-podvm"

    set -x
    exec cloud-api-adaptor alibabacloud \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}
}

gcp() {
    test_vars GCP_CREDENTIALS GCP_PROJECT_ID GCP_ZONE PODVM_IMAGE_NAME

    # Avoid using node's metadata service credentials for GCP authentication
    echo "$GCP_CREDENTIALS" > /tmp/gcp-creds.json
    export GOOGLE_APPLICATION_CREDENTIALS=/tmp/gcp-creds.json

    set -x
    exec cloud-api-adaptor gcp \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}
}

ibmcloud() {
    one_of IBMCLOUD_API_KEY IBMCLOUD_IAM_PROFILE_ID

    set -x
    exec cloud-api-adaptor ibmcloud \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}

}

ibmcloud_powervs() {
    test_vars IBMCLOUD_API_KEY

    set -x
    exec cloud-api-adaptor ibmcloud-powervs \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}

}

libvirt() {
    test_vars LIBVIRT_URI

    set -x
    exec cloud-api-adaptor libvirt \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        -data-dir /opt/data-dir \
        ${optionals}

}

vsphere() {
    test_vars GOVC_USERNAME GOVC_PASSWORD GOVC_URL GOVC_DATACENTER

    set -x
    exec cloud-api-adaptor vsphere \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}

}

docker() {
    set -x
    exec cloud-api-adaptor docker \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}

}

byom() {
    test_vars VM_POOL_IPS

    set -x
    exec cloud-api-adaptor byom \
        -pods-dir "${PEER_PODS_DIR}" \
        -socket "${REMOTE_HYPERVISOR_ENDPOINT}" \
        ${optionals}

}

help_msg() {
    cat <<EOF
Usage:
	CLOUD_PROVIDER=alibabacloud|aws|azure|byom|gcp|ibmcloud|ibmcloud-powervs|libvirt|vsphere|docker $0
or
	$0 alibabacloud|aws|azure|byom|gcp|ibmcloud|ibmcloud-powervs|libvirt|vsphere|docker

in addition all cloud provider specific env variables must be set and valid
(CLOUD_PROVIDER is currently set to "$CLOUD_PROVIDER")
EOF
}

if [[ "$CLOUD_PROVIDER" == "aws" ]]; then
    aws
elif [[ "$CLOUD_PROVIDER" == "azure" ]]; then
    azure
elif [[ "$CLOUD_PROVIDER" == "alibabacloud" ]]; then
    alibabacloud
elif [[ "$CLOUD_PROVIDER" == "byom" ]]; then
    byom
elif [[ "$CLOUD_PROVIDER" == "gcp" ]]; then
    gcp
elif [[ "$CLOUD_PROVIDER" == "ibmcloud" ]]; then
    ibmcloud
elif [[ "$CLOUD_PROVIDER" == "ibmcloud-powervs" ]]; then
    ibmcloud_powervs
elif [[ "$CLOUD_PROVIDER" == "libvirt" ]]; then
    libvirt
elif [[ "$CLOUD_PROVIDER" == "vsphere" ]]; then
    vsphere
elif [[ "$CLOUD_PROVIDER" == "docker" ]]; then
    docker
else
    help_msg
fi
