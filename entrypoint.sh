#!/bin/bash

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER}
CRI_RUNTIME_ENDPOINT=${CRI_RUNTIME_ENDPOINT:-/run/peerpod/cri-runtime.sock}
optionals+=""

# Ensure you add a space before the closing quote (") when updating the optionals
# example: 
# following is the correct method: optionals+="-option val "
# following is the incorrect method: optionals+="-option val"

if [[ -S ${CRI_RUNTIME_ENDPOINT} ]]; then # will skip if socket isn't exist in the container
	optionals+="-cri-runtime-endpoint ${CRI_RUNTIME_ENDPOINT} "
fi

if [[ "${PAUSE_IMAGE}" ]]; then
	optionals+="-pause-image ${PAUSE_IMAGE} "
fi

test_vars() {
        for i in "$@"; do
                [ -z "${!i}" ] && echo "\$$i is NOT set" && EXT=1
        done
        [[ -n $EXT ]] && exit 1
}

aws() {
test_vars AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

[[ "${PODVM_LAUNCHTEMPLATE_NAME}" ]] && optionals+="-use-lt -aws-lt-name ${PODVM_LAUNCHTEMPLATE_NAME} " # has precedence if set
[[ "${AWS_SG_IDS}" ]] && optionals+="-securitygroupids ${AWS_SG_IDS} " # MUST if template is not used
[[ "${PODVM_AMI_ID}" ]] && optionals+="-imageid ${PODVM_AMI_ID} " # MUST if template is not used
[[ "${PODVM_INSTANCE_TYPE}" ]] && optionals+="-instance-type ${PODVM_INSTANCE_TYPE} " # default t3.small
[[ "${SSH_KP_NAME}" ]] && optionals+="-keyname ${SSH_KP_NAME} " # if not retrieved from IMDS
[[ "${AWS_SUBNET_ID}" ]] && optionals+="-subnetid ${AWS_SUBNET_ID} " # if not set retrieved from IMDS
[[ "${AWS_REGION}" ]] && optionals+="-aws-region ${AWS_REGION} " # if not set retrieved from IMDS

set -x
exec cloud-api-adaptor-aws aws \
	-aws-region "${AWS_REGION}" \
	-pods-dir /run/peerpod/pods \
	${optionals} \
	-socket /run/peerpod/hypervisor.sock
}

azure() {
test_vars AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID
set -x

exec cloud-api-adaptor-azure azure \
  -subscriptionid "${AZURE_SUBSCRIPTION_ID}" \
  -region "${AZURE_REGION}" \
  -instance-size "${AZURE_INSTANCE_SIZE}" \
  -resourcegroup "${AZURE_RESOURCE_GROUP}" \
  -subnetid "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}/providers/Microsoft.Network/virtualNetworks/${AZURE_VM_NAME}VNET/subnets/${AZURE_VM_NAME}Subnet" \
  -securitygroupid "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}/providers/Microsoft.Network/networkSecurityGroups/${AZURE_VM_NAME}NSG" \
  -imageid "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}/providers/Microsoft.Compute/images/${AZURE_IMAGE}" \
  ${optionals}
}

ibmcloud() {
test_vars IBMCLOUD_API_KEY
set -x
exec cloud-api-adaptor-ibmcloud ibmcloud \
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
exec cloud-api-adaptor-libvirt libvirt \
	-uri "${LIBVIRT_URI}" \
	-data-dir /opt/data-dir \
	-pods-dir /run/peerpod/pods \
	-network-name "${LIBVIRT_NET:-default}" \
	-pool-name "${LIBVIRT_POOL:-default}" \
	${optionals} \
	-socket /run/peerpod/hypervisor.sock
}

vsphere() {
test_vars GOVC_USERNAME GOVC_PASSWORD

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

exec cloud-api-adaptor-vsphere vsphere \
	-vcenter-url ${GOVC_URL}  \
	${optionals} \
	-socket /run/peerpod/hypervisor.sock
}

help_msg() {
	cat <<EOF
Usage:
	CLOUD_PROVIDER=aws|azure|ibmcloud|libvirt|vsphere $0
or
	$0 aws|azure|ibmcloud|libvirt|vsphere
in addition all cloud provider specific env variables must be set and valid
(CLOUD_PROVIDER is currently set to "$CLOUD_PROVIDER")
EOF
}

if [[ "$CLOUD_PROVIDER" == "aws" ]]; then
	aws
elif [[ "$CLOUD_PROVIDER" == "azure" ]]; then
	azure
elif [[ "$CLOUD_PROVIDER" == "ibmcloud" ]]; then
	ibmcloud
elif [[ "$CLOUD_PROVIDER" == "libvirt" ]]; then
	libvirt
elif [[ "$CLOUD_PROVIDER" == "vsphere" ]]; then
	vsphere
else
	help_msg
fi
