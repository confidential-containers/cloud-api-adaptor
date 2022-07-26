#!/bin/bash
api_key=$IBMCLOUD_API_KEY
ssh_key_id=$SSH_KEY_ID
image_id=$IMAGE_ID
profile=bx2-2x8

vpc_id=$VPC_ID
primary_subnet_id=$VPC_PRIMARY_SUBNET_ID
secondary_subnet_id=$VPC_SECONDARY_SUBNET_ID
primary_security_groupd_id=$VPC_PRIMARY_SECURITY_GROUP_ID
secondary_security_group_id=$VPC_SECONDARY_SECURITY_GROUP_ID

/opt/confidential-containers/bin/cloud-api-adaptor-ibmcloud ibmcloud \
    -api-key "$api_key" \
    -key-id "$ssh_key_id" \
    -image-id "$image_id" \
    -profile-name "$profile" \
    -zone-name jp-tok-2 \
    -primary-subnet-id "$primary_subnet_id" \
    -primary-security-group-id "$primary_security_groupd_id" \
    -secondary-subnet-id "$secondary_subnet_id" \
    -secondary-security-group-id "$secondary_security_group_id" \
    -vpc-id "$vpc_id" \
    -pods-dir /run/peerpod/pods \
    -socket /run/peerpod/hypervisor.sock \
