#!/bin/bash

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER}

aws() {
set -x
cloud-api-adaptor aws \
	-aws-access-key-id ${AWS_ACCESS_KEY_ID} \
	-aws-secret-key ${AWS_SECRET_ACCESS_KEY} \
	-aws-region ${AWS_REGION} \
	-pods-dir /run/peerpod/pods \
	-socket /run/peerpod/hypervisor.sock
}

libvirt() {
set -x
cloud-api-adaptor-libvirt libvirt \
	-uri ${LIBVIRT_URI} \
	-data-dir /opt/data-dir \
	-pods-dir /run/peerpod/pods \
	-network-name ${LIBVIRT_NET:-default} \
	-pool-name ${LIBVIRT_POOL:-default} \
	-socket /run/peerpod/hypervisor.sock
}

ibmcloud() {
set -x
cloud-api-adaptor ibmcloud \
	-api-key ${IBMCLOUD_API_KEY} \
	-key-id ${SSH_KEY_ID} \
	-image-id ${IMAGE_ID} \
	-profile-name bx2-2x8 \
	-zone-name jp-tok-2 \
	-primary-subnet-id ${VPC_PRIMARY_SUBNET_ID} \
	-primary-security-group-id ${VPC_PRIMARY_SECURITY_GROUP_ID} \
	-secondary-subnet-id ${VPC_SECONDARY_SECURITY_SUBNET_ID} \
	-secondary-security-group-id ${VPC_SECONDARY_SECURITY_GROUP_ID} \
	-vpc-id ${VPC_ID} \
	-pods-dir /run/peerpod/pods \
	-socket /run/peerpod/hypervisor.sock
}

if [[ "$CLOUD_PROVIDER" == "aws" ]]; then
	aws
elif [[ "$CLOUD_PROVIDER" == "libvirt" ]]; then
	libvirt
elif [[ "$CLOUD_PROVIDER" == "ibmcloud" ]]; then
	ibmcloud
else
	cat <<EOF
Usage:
	CLOUD_PROVIDER=aws|libvirt|ibmcloud $0
or
	$0 aws|libvirt|ibmcloud
in addition all cloud provider specific env variables must be set and valid
(CLOUD_PROVIDER is currently set to "$CLOUD_PROVIDER")
EOF
fi
