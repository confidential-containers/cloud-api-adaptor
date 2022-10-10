#!/bin/bash

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER}
CRI_RUNTIME_ENDPOINT=${CRI_RUNTIME_ENDPOINT:-/run/peerpod/cri-runtime.sock}
optionals+=""

if [[ -S ${CRI_RUNTIME_ENDPOINT} ]]; then # will skip if socket isn't exist in the container
	optionals+="-cri-runtime-endpoint ${CRI_RUNTIME_ENDPOINT} "
fi

if [[ "${PAUSE_IMAGE}" ]]; then
	optionals+="-pause-image ${PAUSE_IMAGE}"
fi

test_vars() {
        for i in "$@"; do
                [ -z "${!i}" ] && echo "\$$i is NOT set" && EXT=1
        done
        [[ -n $EXT ]] && exit 1
}

aws() {
set -x

if [[ "${PODVM_LAUNCHTEMPLATE_NAME}" ]]; then
	optionals+="-use-lt -aws-lt-name ${PODVM_LAUNCHTEMPLATE_NAME}"
else
	optionals+="-imageid ${PODVM_AMI_ID} "
	optionals+="-instance-type ${PODVM_INSTANCE_TYPE:-t3.small} "
	optionals+="-securitygroupid ${AWS_SG_ID} "
	optionals+="-keyname ${SSH_KP_NAME} "
	optionals+="-subnetid ${AWS_SUBNET_ID} "
fi

cloud-api-adaptor-aws aws \
	-aws-access-key-id "${AWS_ACCESS_KEY_ID}" \
	-aws-secret-key "${AWS_SECRET_ACCESS_KEY}" \
	-aws-region "${AWS_REGION}" \
	-pods-dir /run/peerpod/pods \
	${optionals} \
	-socket /run/peerpod/hypervisor.sock
}

ibmcloud() {
set -x
cloud-api-adaptor-ibmcloud ibmcloud \
        -api-key "${IBMCLOUD_API_KEY}" \
        -iam-service-url "${IBMCLOUD_IAM_ENDPOINT}" \
        -vpc-service-url  "${IBMCLOUD_VPC_ENDPOINT}" \
        -resource-group-id "${IBMCLOUD_RESOURCE_GROUP_ID}" \
        -key-id "${IBMCLOUD_SSH_KEY_ID}" \
        -image-id "${IBMCLOUD_PODVM_IMAGE_ID}" \
        -profile-name "${IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME}" \
        -zone-name "${IBMCLOUD_ZONE}" \
        -primary-subnet-id "${IBMCLOUD_VPC_SUBNET_ID}" \
        -primary-security-group-id "${IBMCLOUD_VPC_SG_ID}" \
        -vpc-id "${IBMCLOUD_VPC_ID}" \
        -pods-dir /run/peerpod/pods \
	${optionals} \
        -socket /run/peerpod/hypervisor.sock
}

libvirt() {
test_vars LIBVIRT_URI
set -x
cloud-api-adaptor-libvirt libvirt \
	-uri "${LIBVIRT_URI}" \
	-data-dir /opt/data-dir \
	-pods-dir /run/peerpod/pods \
	-network-name "${LIBVIRT_NET:-default}" \
	-pool-name "${LIBVIRT_POOL:-default}" \
	${optionals} \
	-socket /run/peerpod/hypervisor.sock
}

vsphere() {
set -x

if [[ "${GOVC_TEMPLATE}" ]]; then
    optionals+="-template ${GOVC_TEMPLATE} "
fi

if [[ "${GOVC_DATACENTER}" ]]; then
    optionals+="-data-center ${GOVC_DATACENTER} "
fi

if [[ "${GOVC_VCLUSTER}" ]]; then
    optionals+="-vcluster ${GOVC_VCLUSTER} "
fi

if [[ "${GOVC_DATASTORE}" ]]; then
    optionals+="-data-store ${GOVC_DATASTORE} "
fi

if [[ "${GOVC_RESOURCE_POOL}" ]]; then
    optionals+="-resource-pool ${GOVC_RESOURCE_POOL} "
fi

if [[ "${GOVC_FOLDER}" ]]; then
    optionals+="-deploy-folder ${GOVC_FOLDER} "
fi

cloud-api-adaptor-vsphere vsphere \
	-vcenter-url ${GOVC_URL}  \
	-user-name ${GOVC_USERNAME} \
	-password ${GOVC_PASSWORD} \
	${optionals} \
	-socket /run/peerpod/hypervisor.sock
}

help_msg() {
	cat <<EOF
Usage:
	CLOUD_PROVIDER=aws|ibmcloud|libvirt|vsphere $0
or
	$0 aws|ibmcloud|libvirt|vsphere
in addition all cloud provider specific env variables must be set and valid
(CLOUD_PROVIDER is currently set to "$CLOUD_PROVIDER")
EOF
}

if [[ "$CLOUD_PROVIDER" == "aws" ]]; then
	aws
elif [[ "$CLOUD_PROVIDER" == "ibmcloud" ]]; then
	ibmcloud
elif [[ "$CLOUD_PROVIDER" == "libvirt" ]]; then
	libvirt
elif [[ "$CLOUD_PROVIDER" == "vsphere" ]]; then
	vsphere
else
	help_msg
fi
