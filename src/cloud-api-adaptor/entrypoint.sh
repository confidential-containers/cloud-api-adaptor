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

optionals+=""

# Note: Most common flags (PAUSE_IMAGE, TUNNEL_TYPE, VXLAN_PORT, etc.) are now
# handled directly by Go code via FlagRegistrar in main.go and no longer need
# env-to-arg conversion here.

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
    exec cloud-api-adaptor aws ${optionals}

}

azure() {
    test_vars AZURE_CLIENT_ID AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID AZURE_RESOURCE_GROUP AZURE_SUBNET_ID AZURE_IMAGE_ID

    set -x
    exec cloud-api-adaptor azure ${optionals}
}

alibabacloud() {
    one_of ALIBABACLOUD_ACCESS_KEY_ID ALIBABA_CLOUD_ROLE_ARN

    # TODO: Variable name mismatch - kustomization/entrypoint uses INSTANCE_TYPE
    # but manager.go expects PODVM_INSTANCE_TYPE. Consider standardizing in future.
    [[ "${INSTANCE_TYPE}" ]] && optionals+=" -instance-type ${INSTANCE_TYPE} "
    # Global flag without env var support - still need conversion
    [[ "${EXTERNAL_NETWORK_VIA_PODVM}" ]] && optionals+=" -ext-network-via-podvm"

    set -x
    exec cloud-api-adaptor alibabacloud ${optionals}
}

gcp() {
    test_vars GCP_CREDENTIALS GCP_PROJECT_ID GCP_ZONE PODVM_IMAGE_NAME

    # Avoid using node's metadata service credentials for GCP authentication
    echo "$GCP_CREDENTIALS" > /tmp/gcp-creds.json
    export GOOGLE_APPLICATION_CREDENTIALS=/tmp/gcp-creds.json

    set -x
    exec cloud-api-adaptor gcp ${optionals}
}

ibmcloud() {
    one_of IBMCLOUD_API_KEY IBMCLOUD_IAM_PROFILE_ID

    set -x
    exec cloud-api-adaptor ibmcloud ${optionals}

}

ibmcloud_powervs() {
    test_vars IBMCLOUD_API_KEY

    set -x
    exec cloud-api-adaptor ibmcloud-powervs ${optionals}

}

libvirt() {
    test_vars LIBVIRT_URI

    set -x
    exec cloud-api-adaptor libvirt -data-dir /opt/data-dir ${optionals}

}

vsphere() {
    test_vars GOVC_USERNAME GOVC_PASSWORD GOVC_URL GOVC_DATACENTER

    set -x
    exec cloud-api-adaptor vsphere ${optionals}

}

docker() {
    set -x
    exec cloud-api-adaptor docker ${optionals}

}

byom() {
    test_vars VM_POOL_IPS

    set -x
    exec cloud-api-adaptor byom ${optionals}

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
