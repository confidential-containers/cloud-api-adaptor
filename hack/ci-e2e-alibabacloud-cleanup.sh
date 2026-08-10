#!/bin/bash
#
# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Primarily used on Github workflows to remove dangling resources from Alibaba Cloud
#

script_dir=$(cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)

if [ -z "${RESOURCES_BASENAME:-}" ]; then
  echo "RESOURCES_BASENAME variable is not exported"
  exit 1
fi

REGION="${REGION:-cn-hongkong}"

delete_vpcs() {
  local tag_vpc="${RESOURCES_BASENAME}-vpc"
  local vpc_ids
  vpc_ids=$(aliyun vpc DescribeVpcs --RegionId "$REGION" \
    --query "Vpcs.Vpc[?VpcName=='${tag_vpc}'].VpcId" --output text 2>/dev/null || true)

  if [ -z "$vpc_ids" ]; then
    echo "There aren't VPCs to delete"
    return
  fi

  for vpc in $vpc_ids; do
    echo "alibabacloud_vpc_id=\"$vpc\"" > "$TEST_PROVISION_FILE"
    echo "region=\"$REGION\"" >> "$TEST_PROVISION_FILE"
    echo "resources_basename=\"$RESOURCES_BASENAME\"" >> "$TEST_PROVISION_FILE"

    local vswitches
    vswitches=$(aliyun vpc DescribeVSwitches --RegionId "$REGION" --VpcId "$vpc" \
      --query 'VSwitches.VSwitch[*].VSwitchId' --output text 2>/dev/null || true)
    for vs in $vswitches; do
      echo "alibabacloud_vpc_vswitch_id=\"$vs\"" >> "$TEST_PROVISION_FILE"
    done

    local sgs
    sgs=$(aliyun ecs DescribeSecurityGroups --RegionId "$REGION" --VpcId "$vpc" \
      --SecurityGroupName "${RESOURCES_BASENAME}-sg" \
      --query 'SecurityGroups.SecurityGroup[*].SecurityGroupId' --output text 2>/dev/null || true)
    for sg in $sgs; do
      echo "alibabacloud_vpc_sg_id=\"$sg\"" >> "$TEST_PROVISION_FILE"
    done

    echo "Delete VPC=$vpc"
    ./caa-provisioner-cli -action deprovision
  done
}

delete_images() {
  local tag_img="${RESOURCES_BASENAME}-img"
  local image_ids
  image_ids=$(aliyun ecs DescribeImages --RegionId "$REGION" --ImageOwnerAlias self \
    --query "Images.Image[?Tags.Tag[?TagKey=='Name' && TagValue=='${tag_img}']].ImageId" \
    --output text 2>/dev/null || true)

  if [ -z "$image_ids" ]; then
    # Fallback: match by image name prefix
    image_ids=$(aliyun ecs DescribeImages --RegionId "$REGION" --ImageOwnerAlias self \
      --query "Images.Image[?starts_with(ImageName, 'podvm-')].ImageId" \
      --output text 2>/dev/null || true)
  fi

  if [ -z "$image_ids" ]; then
    echo "There aren't custom images to delete."
    return
  fi

  for image in $image_ids; do
    echo "Deleting custom image: $image"
    aliyun ecs DeleteImage --RegionId "$REGION" --ImageId "$image" --Force true 2>/dev/null || true
  done
}

delete_oss_buckets() {
  local bucket_prefix
  bucket_prefix=$(echo "${RESOURCES_BASENAME}-bucket" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9-' '-')
  local endpoint="oss-${REGION}.aliyuncs.com"

  # List buckets and delete matching ones
  local buckets
  buckets=$(aliyun oss ls --endpoint "$endpoint" 2>/dev/null | awk '{print $NF}' | grep -F "$bucket_prefix" || true)

  if [ -z "$buckets" ]; then
    echo "There aren't OSS buckets to delete."
    return
  fi

  for bucket in $buckets; do
    # strip oss:// prefix if present
    bucket=${bucket#oss://}
    echo "Deleting OSS bucket: $bucket"
    aliyun oss rm "oss://${bucket}" --recursive --force --endpoint "$endpoint" 2>/dev/null || true
    aliyun oss rm "oss://${bucket}" --bucket --force --endpoint "$endpoint" 2>/dev/null || true
  done
}

main() {
  TEST_PROVISION_FILE="$(pwd)/alibabacloud.properties"
  export TEST_PROVISION_FILE
  export REGION

  CLOUD_PROVIDER="alibabacloud"
  export CLOUD_PROVIDER

  echo "Build the caa-provisioner-cli tool"
  cd "${script_dir}/../src/cloud-api-adaptor/test/tools" || exit 1
  make

  if ! command -v aliyun >/dev/null 2>&1; then
    echo "Installing aliyun CLI"
    tmp="$(mktemp -d)"
    curl -fsSL -o "${tmp}/aliyun.tgz" "https://aliyuncli.alicdn.com/aliyun-cli-linux-latest-amd64.tgz"
    tar -xzf "${tmp}/aliyun.tgz" -C "${tmp}"
    sudo mv "${tmp}/aliyun" /usr/local/bin/aliyun
    rm -rf "${tmp}"
  fi

  delete_vpcs
  delete_images
  delete_oss_buckets
}

main
