#!/bin/bash

/opt/confidential-containers/bin/cloud-api-adaptor-ibmcloud ibmcloud \
    -api-key "${IBMCLOUD_API_KEY}" \
    -key-id "${SSH_KEY_ID}" \
    -image-id "${IMAGE_ID}" \
    -profile-name "${INSTANCE_PROFILE}" \
    -zone-name ${IBMCLOUD_ZONE} \
    -primary-subnet-id "${VPC_PRIMARY_SUBNET_ID}" \
    -secondary-subnet-id "${VPC_SECONDARY_SUBNET_ID}" \
    -primary-security-group-id "${VPC_PRIMARY_SECURITY_GROUP_ID}" \
    -secondary-security-group-id "${VPC_SECONDARY_SECURITY_GROUP_ID}" \
    -vpc-id "${VPC_ID}" \
    -pods-dir /run/peerpod/pods \
    -socket /run/peerpod/hypervisor.sock \
    -cri-runtime-endpoint "${CRI_RUNTIME_ENDPOINT}"
