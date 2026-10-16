#!/bin/bash
#
# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Remove Alibaba Cloud resources left behind by e2e runs.
#
# Alibaba Cloud CLI does not accept AWS-style --query / --output text. Those
# flags used to fail, the errors were discarded, and VPC / vSwitch cleanup
# never ran. Leftover ci-caa-* VPCs then filled the regional VPC quota
# (QuotaExceeded.Vpc) so later nightly runs could not create a new VPC.
#
# Deletes:
#   - resources belonging to this GitHub run (RESOURCES_BASENAME, including
#     earlier attempts of the same run id)
#   - other ci-caa-* resources older than STALE_MINUTES (default 120)
#
# Set STALE_MINUTES=0 to reap every ci-caa-* resource in the region.

set -uo pipefail

REGION="${REGION:-cn-hongkong}"
STALE_MINUTES="${STALE_MINUTES:-120}"
# ci-caa-<run_id>-  (trailing dash so run 222 does not match run 2220)
if [ -n "${RESOURCES_BASENAME:-}" ]; then
  RUN_PREFIX="${RESOURCES_BASENAME%-*}-"
else
  RUN_PREFIX=""
fi

should_reap() {
  local name="$1"
  local created="$2"
  local pattern="$3"

  case "$name" in
    "$pattern") ;;
    *) return 1 ;;
  esac
  if [ -n "$RUN_PREFIX" ]; then
    case "$name" in
      "${RUN_PREFIX}"*) return 0 ;;
    esac
  fi

  local epoch now age
  epoch=$(date -d "$created" +%s 2>/dev/null || true)
  if [ -z "$epoch" ]; then
    echo "Skip ${name}: cannot parse creation time '${created}'" >&2
    return 1
  fi
  now=$(date +%s)
  age=$(( (now - epoch) / 60 ))
  [ "$age" -ge "$STALE_MINUTES" ]
}

self_test() {
  local old now_iso fail=0
  old=$(date -u -d '3 hours ago' +%Y-%m-%dT%H:%M:%SZ)
  now_iso=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  RESOURCES_BASENAME="ci-caa-222-1"
  RUN_PREFIX="ci-caa-222-"
  STALE_MINUTES=120

  should_reap "ci-caa-222-1-vpc" "$now_iso" 'ci-caa-*-vpc' || fail=1
  should_reap "ci-caa-222-2-vpc" "$now_iso" 'ci-caa-*-vpc' || fail=1
  should_reap "ci-caa-111-1-vpc" "$old" 'ci-caa-*-vpc' || fail=1
  if should_reap "ci-caa-111-1-vpc" "$now_iso" 'ci-caa-*-vpc'; then fail=1; fi
  if should_reap "ci-caa-2220-1-vpc" "$now_iso" 'ci-caa-*-vpc'; then fail=1; fi
  if should_reap "prod-vpc" "$old" 'ci-caa-*-vpc'; then fail=1; fi
  if [ "$fail" -ne 0 ]; then
    echo "should_reap self-test failed" >&2
    return 1
  fi
  echo "should_reap self-test ok"
}

aliyun_json() {
  local err out
  err=$(mktemp)
  if ! out=$(aliyun "$@" --region "$REGION" 2>"$err"); then
    echo "aliyun $* failed: $(cat "$err")" >&2
    rm -f "$err"
    return 1
  fi
  rm -f "$err"
  if [ -z "$out" ]; then
    echo "aliyun $* returned an empty body" >&2
    return 1
  fi
  printf '%s' "$out"
}

# Print a JSON array of every page. jq_path selects the page's object array.
list_pages() {
  local product="$1"
  local api="$2"
  local jq_path="$3"
  shift 3

  local page=1 acc='[]' page_size=50
  while [ "$page" -le 40 ]; do
    local resp batch count total got
    if ! resp=$(aliyun_json "$product" "$api" "$@" --PageNumber "$page" --PageSize "$page_size"); then
      return 1
    fi
    batch=$(jq -c "${jq_path} // []" <<<"$resp") || return 1
    count=$(jq 'length' <<<"$batch")
    total=$(jq -r 'if .TotalCount == null then "" else .TotalCount end' <<<"$resp")
    acc=$(jq -c --argjson a "$acc" --argjson b "$batch" -n '$a + $b')
    got=$(jq 'length' <<<"$acc")
    if [ "$count" -eq 0 ]; then
      break
    fi
    if [ -n "$total" ] && [ "$got" -ge "$total" ]; then
      break
    fi
    if [ "$count" -lt "$page_size" ]; then
      break
    fi
    page=$((page + 1))
  done
  printf '%s' "$acc"
}

ignore_missing() {
  local err
  err=$(mktemp)
  if aliyun "$@" --region "$REGION" >"$err" 2>&1; then
    rm -f "$err"
    return 0
  fi
  if grep -Eq 'NotFound|InvalidVSwitchId|InvalidVpcId|InvalidSecurityGroupId|InvalidInstanceId|InvalidEniId|InvalidNetworkInterface' "$err"; then
    rm -f "$err"
    return 0
  fi
  cat "$err" >&2
  rm -f "$err"
  return 1
}

delete_instances() {
  local vpc="$1"
  local deadline=$((SECONDS + 480))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local instances
    if ! instances=$(list_pages ecs DescribeInstances '.Instances.Instance' --RegionId "$REGION" --VpcId "$vpc"); then
      echo "Failed to list instances in ${vpc}" >&2
      return 1
    fi
    local count
    count=$(jq 'length' <<<"$instances")
    if [ "$count" -eq 0 ]; then
      return 0
    fi
    echo "VPC ${vpc} still has ${count} instance(s); deleting"
    local id
    while IFS= read -r id; do
      [ -n "$id" ] || continue
      echo "Delete instance ${id}"
      ignore_missing ecs DeleteInstance --InstanceId "$id" --Force true || true
    done < <(jq -r '.[].InstanceId // empty' <<<"$instances")
    sleep 15
  done
  echo "Timed out waiting for instances in ${vpc} to be deleted" >&2
  return 1
}

delete_enis() {
  local vsw="$1"
  local enis
  if ! enis=$(list_pages ecs DescribeNetworkInterfaces '.NetworkInterfaceSets.NetworkInterfaceSet' --RegionId "$REGION" --VSwitchId "$vsw"); then
    echo "Failed to list ENIs on vSwitch ${vsw}" >&2
    return 1
  fi
  local id typ
  while IFS=$'\t' read -r id typ; do
    [ -n "$id" ] || continue
    echo "Delete ENI ${id} (${typ}) on vSwitch ${vsw}"
    # Primary ENIs disappear with the instance. A leftover Available ENI still
    # blocks DeleteVSwitch, so try every ENI and ignore the ones still in use.
    ignore_missing ecs DeleteNetworkInterface --RegionId "$REGION" --NetworkInterfaceId "$id" || true
  done < <(jq -r '.[] | [(.NetworkInterfaceId // ""), (.Type // "")] | @tsv' <<<"$enis")
}

vswitch_gone() {
  local id="$1"
  local err
  err=$(mktemp)
  if aliyun vpc DescribeVSwitchAttributes --RegionId "$REGION" --VSwitchId "$id" --region "$REGION" >"$err" 2>&1; then
    rm -f "$err"
    return 1
  fi
  if grep -Eq 'NotFound|InvalidVSwitchId' "$err"; then
    rm -f "$err"
    return 0
  fi
  cat "$err" >&2
  rm -f "$err"
  return 1
}

# DeleteVSwitch returns before the switch is gone, and DeleteVpc fails until then.
wait_vswitch_gone() {
  local id="$1"
  local deadline=$((SECONDS + 300))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if vswitch_gone "$id"; then
      return 0
    fi
    sleep 5
  done
  echo "Timed out waiting for vSwitch ${id} to be deleted" >&2
  return 1
}

delete_vswitches() {
  local vpc="$1"
  local switches
  if ! switches=$(list_pages vpc DescribeVSwitches '.VSwitches.VSwitch' --RegionId "$REGION" --VpcId "$vpc"); then
    echo "Failed to list vSwitches in ${vpc}" >&2
    return 1
  fi
  local failed=0
  local id
  while IFS= read -r id; do
    [ -n "$id" ] || continue
    delete_enis "$id" || true
    echo "Delete vSwitch ${id}"
    local attempt
    for attempt in 1 2 3 4 5 6; do
      if ignore_missing vpc DeleteVSwitch --RegionId "$REGION" --VSwitchId "$id"; then
        break
      fi
      echo "vSwitch ${id} still in use, retry ${attempt}" >&2
      sleep 10
    done
    wait_vswitch_gone "$id" || failed=1
  done < <(jq -r '.[].VSwitchId // empty' <<<"$switches")
  return "$failed"
}

delete_security_groups() {
  local vpc="$1"
  local groups
  if ! groups=$(list_pages ecs DescribeSecurityGroups '.SecurityGroups.SecurityGroup' --RegionId "$REGION" --VpcId "$vpc"); then
    echo "Failed to list security groups in ${vpc}" >&2
    return 1
  fi
  local id managed name
  while IFS=$'\t' read -r id managed name; do
    [ -n "$id" ] || continue
    if [ "$managed" = "true" ]; then
      continue
    fi
    echo "Delete security group ${id} (${name})"
    # The VPC default group cannot be deleted by itself; DeleteVpc removes it.
    ignore_missing ecs DeleteSecurityGroup --RegionId "$REGION" --SecurityGroupId "$id" || true
  done < <(jq -r '.[] | [(.SecurityGroupId // ""), (.ServiceManaged // false | tostring), (.SecurityGroupName // "")] | @tsv' <<<"$groups")
}

delete_one_vpc() {
  local vpc="$1"
  local name="$2"
  echo "Delete VPC ${vpc} (${name})"
  delete_instances "$vpc" || true
  delete_vswitches "$vpc" || true
  delete_security_groups "$vpc" || true

  local attempt
  for attempt in 1 2 3 4 5 6 7 8; do
    if ignore_missing vpc DeleteVpc --RegionId "$REGION" --VpcId "$vpc"; then
      echo "Deleted VPC ${vpc}"
      return 0
    fi
    echo "VPC ${vpc} still has dependents, retry ${attempt}" >&2
    # Switches were already waited on above. Ask again, but do not block for
    # another five minutes on every VPC retry.
    local switches id
    if switches=$(list_pages vpc DescribeVSwitches '.VSwitches.VSwitch' --RegionId "$REGION" --VpcId "$vpc"); then
      while IFS= read -r id; do
        [ -n "$id" ] || continue
        ignore_missing vpc DeleteVSwitch --RegionId "$REGION" --VSwitchId "$id" || true
      done < <(jq -r '.[].VSwitchId // empty' <<<"$switches")
    fi
    delete_security_groups "$vpc" || true
    sleep 10
  done
  echo "Failed to delete VPC ${vpc}" >&2
  return 1
}

delete_vpcs() {
  local vpcs
  if ! vpcs=$(list_pages vpc DescribeVpcs '.Vpcs.Vpc' --RegionId "$REGION"); then
    echo "Failed to list VPCs in ${REGION}" >&2
    return 1
  fi
  local total
  total=$(jq 'length' <<<"$vpcs")
  echo "Region ${REGION} has ${total} VPC(s); reaping ci-caa leftovers (stale after ${STALE_MINUTES}m, run prefix '${RUN_PREFIX:-<none>}')"

  local failed=0
  local id name created
  while IFS=$'\t' read -r id name created; do
    [ -n "$id" ] || continue
    if should_reap "$name" "$created" 'ci-caa-*-vpc'; then
      delete_one_vpc "$id" "$name" || failed=1
    else
      echo "Keep VPC ${id} (${name}, created ${created})"
    fi
  done < <(jq -r '.[] | [(.VpcId // ""), (.VpcName // ""), (.CreationTime // "")] | @tsv' <<<"$vpcs")
  return "$failed"
}

delete_images() {
  local images
  if ! images=$(list_pages ecs DescribeImages '.Images.Image' --RegionId "$REGION" --ImageOwnerAlias self); then
    echo "Failed to list custom images; skipping" >&2
    return 0
  fi
  local id name created
  while IFS=$'\t' read -r id name created; do
    [ -n "$id" ] || continue
    if should_reap "$name" "$created" 'ci-caa-*-img'; then
      echo "Delete image ${id} (Name=${name})"
      ignore_missing ecs DeleteImage --RegionId "$REGION" --ImageId "$id" --Force true || true
    fi
  done < <(jq -r '
    .[] | . as $img
    | ($img.Tags.Tag // [])[]?
    | select(.TagKey == "Name")
    | [($img.ImageId // ""), (.TagValue // ""), ($img.CreationTime // "")]
    | @tsv
  ' <<<"$images")
}

delete_oss_buckets() {
  local listing
  listing=$(aliyun oss ls 2>/dev/null || true)
  if [ -z "$listing" ]; then
    echo "No OSS buckets listed; skipping"
    return 0
  fi

  local targets
  targets=$(OSS_LISTING="$listing" RUN_PREFIX="$RUN_PREFIX" STALE_MINUTES="$STALE_MINUTES" REGION="$REGION" python3 - <<'PY'
import os, re
from datetime import datetime, timezone

stale = int(os.environ.get("STALE_MINUTES", "120"))
prefix = os.environ.get("RUN_PREFIX", "")
default_region = os.environ.get("REGION", "cn-hongkong")
now = datetime.now(timezone.utc)

for raw in os.environ.get("OSS_LISTING", "").splitlines():
    line = raw.strip()
    bucket_m = re.search(r"oss://(\S+)", line)
    if not bucket_m:
        continue
    bucket = bucket_m.group(1)
    if not bucket.startswith("ci-caa-") or not bucket.endswith("-bucket"):
        continue
    region_m = re.search(r"\b(oss-[a-z0-9-]+)\b", line)
    region = region_m.group(1)[4:] if region_m else default_region
    reap = bool(prefix) and bucket.startswith(prefix)
    if not reap:
        time_m = re.match(r"(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} [+-]\d{4})", line)
        if not time_m:
            print(f"Skip bucket {bucket}: cannot parse creation time", file=__import__("sys").stderr)
            continue
        created = datetime.strptime(time_m.group(1), "%Y-%m-%d %H:%M:%S %z")
        age = (now - created).total_seconds() / 60
        reap = age >= stale
    if reap:
        print(f"{region}\t{bucket}")
PY
)
  if [ -z "$targets" ]; then
    echo "No ci-caa OSS buckets to delete"
    return 0
  fi
  local region bucket
  while IFS=$'\t' read -r region bucket; do
    [ -n "$bucket" ] || continue
    local endpoint="oss-${region}.aliyuncs.com"
    echo "Delete OSS bucket ${bucket} (${endpoint})"
    aliyun oss rm "oss://${bucket}" --recursive --force --endpoint "$endpoint" >/dev/null 2>&1 || true
    aliyun oss rm "oss://${bucket}" --bucket --force --endpoint "$endpoint" >/dev/null 2>&1 || true
  done <<<"$targets"
}

ensure_tools() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "Installing jq"
    sudo apt-get update -y
    sudo apt-get install -y jq
  fi
  if ! command -v aliyun >/dev/null 2>&1; then
    echo "Installing aliyun CLI"
    local tmp
    tmp="$(mktemp -d)"
    curl -fsSL -o "${tmp}/aliyun.tgz" "https://aliyuncli.alicdn.com/aliyun-cli-linux-latest-amd64.tgz"
    tar -xzf "${tmp}/aliyun.tgz" -C "${tmp}"
    sudo mv "${tmp}/aliyun" /usr/local/bin/aliyun
    rm -rf "${tmp}"
  fi
  if ! command -v python3 >/dev/null 2>&1; then
    echo "python3 is required to parse OSS listings" >&2
    return 1
  fi
}

main() {
  if [ "${1:-}" = "--self-test" ]; then
    self_test
    return
  fi

  echo "Alibaba Cloud cleanup region=${REGION} resources_basename=${RESOURCES_BASENAME:-} stale_minutes=${STALE_MINUTES}"
  ensure_tools || return 1

  local failed=0
  delete_vpcs || failed=1
  delete_images || true
  delete_oss_buckets || true
  if [ "$failed" -ne 0 ]; then
    echo "Some VPCs could not be deleted; see the log above" >&2
    return 1
  fi
}

main "$@"
