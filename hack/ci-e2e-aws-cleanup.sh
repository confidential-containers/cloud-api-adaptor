#!/bin/bash
#
# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Primarily used on Github workflows to remove dangling resources from AWS
#

script_dir=$(cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)

if [ -z "${RESOURCES_BASENAME:-}" ]; then
  echo "RESOURCES_BASENAME variable is not exported"
  exit 1
fi

delete_vpcs() {
  local tag_vpc="${RESOURCES_BASENAME}-vpc"
  read -r -a vpcs <<< "$(aws  ec2 describe-vpcs --filters Name=tag:Name,Values=$tag_vpc --query 'Vpcs[*].VpcId' --output text)"

  if [ ${#vpcs[@]} -eq 0 ]; then
    echo "There aren't VPCs to delete"
    return
  fi

  for vpc in "${vpcs[@]}"; do
    echo "aws_vpc_id=\"$vpc\"" > "$TEST_PROVISION_FILE"

    # Find related subnets
    read -r -a subnets <<< "$(aws ec2 describe-subnets --filter "Name=vpc-id,Values=$vpc" --query 'Subnets[*].SubnetId' --output text)"
    for net in "${subnets[@]}"; do
      echo "aws_vpc_subnet_id=\"$net\"" >> "$TEST_PROVISION_FILE"
    done

    # Find related security groups
    read -r -a sgs <<< "$(aws ec2 describe-security-groups --filters "Name=vpc-id,Values=$vpc" "Name=tag:Name,Values=${RESOURCES_BASENAME}-sg" --query 'SecurityGroups[*].GroupId' --output text)"
    for sg in "${sgs[@]}"; do
      echo "aws_vpc_sg_id=\"$sg\"" >> "$TEST_PROVISION_FILE"
    done

    # Find related route tables and internet gateways
    read -r -a rtbs <<< "$(aws ec2 describe-route-tables --filters "Name=vpc-id,Values=$vpc" "Name=tag:Name,Values=${RESOURCES_BASENAME}-rtb" --query 'RouteTables[*].RouteTableId' --output text)"
    for rtb in "${rtbs[@]}"; do
      echo "aws_vpc_rt_id=\"$rtb\"" >> "$TEST_PROVISION_FILE"
      read -r -a igws <<< "$(aws ec2 describe-route-tables --filter "Name=route-table-id,Values=$rtb" --query 'RouteTables[0].Routes[*].GatewayId' --output text)"
      for igw in "${igws[@]}"; do
        [ "$igw" != "local" ] && echo "aws_vpc_igw_id=\"$igw\"" >> "$TEST_PROVISION_FILE"
      done
    done

    echo "Delete VPC=$vpc"
    ./caa-provisioner-cli -action deprovision
  done
}

delete_amis() {
  local tag_ami="${RESOURCES_BASENAME}-img"

  read -r -a amis <<< "$(aws ec2 describe-images --owners self --filters "Name=tag:Name,Values=$tag_ami" --query 'Images[*].ImageId' --output text)"

  if [ ${#amis[@]} -eq 0 ]; then
    echo "There aren't AMIs to delete."
    return
  fi

  for ami in "${amis[@]}"; do
    echo "Deregistering AMI: $ami"
    # Find related snapshots
    snap_ids=$(aws ec2 describe-images --image-ids "$ami" --query 'Images[*].BlockDeviceMappings[*].Ebs.SnapshotId' --output text)
    aws ec2 deregister-image --image-id "$ami"
    for snap in $snap_ids; do
      echo "Deleting snapshot: $snap"
      aws ec2 delete-snapshot --snapshot-id "$snap"
    done
  done

  # Delete the vmimport role if it exists
  local vmimport_role="${RESOURCES_BASENAME}-vmimport"
  if aws iam get-role --role-name "$vmimport_role" >/dev/null 2>&1; then
    echo "Deleting vmimport role: $vmimport_role"
    # First delete the role policy
    aws iam delete-role-policy --role-name "$vmimport_role" --policy-name "vmimport" 2>/dev/null || true
    # Then delete the role
    aws iam delete-role --role-name "$vmimport_role" 2>/dev/null || true
  fi
}

delete_s3_buckets() {
  local tag_bucket="${RESOURCES_BASENAME}-bucket"

  # List all buckets and find ones that match our naming pattern
  read -r -a buckets <<< "$(aws s3api list-buckets --query "Buckets[?contains(Name, '${tag_bucket}')].Name" --output text)"

  if [ ${#buckets[@]} -eq 0 ]; then
    echo "There aren't S3 buckets to delete."
    return
  fi

  for bucket in "${buckets[@]}"; do
    echo "Deleting S3 bucket: $bucket"
    # First, delete all objects in the bucket
    aws s3 rm "s3://$bucket" --recursive 2>/dev/null || true
    # Then delete the bucket
    aws s3api delete-bucket --bucket "$bucket" 2>/dev/null || true
  done
}

main() {
  TEST_PROVISION_FILE="$(pwd)/aws.properties"
  export TEST_PROVISION_FILE

  CLOUD_PROVIDER="aws"
  export CLOUD_PROVIDER

  echo "Build the caa-provisioner-cli tool"
  cd "${script_dir}/../src/cloud-api-adaptor/test/tools" || exit 1
  make

  delete_vpcs
  delete_amis
  delete_s3_buckets
}

main