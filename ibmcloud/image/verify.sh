#!/bin/bash
#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit -o pipefail -o nounset

cd "$(dirname "${BASH_SOURCE[0]}")"

function usage() {
    echo "Usage: $0 --image <image name>"
}

while (( $# )); do
    case "$1" in
        --image)    image=$2 ;;
        --help)     usage; exit 0 ;;
        *)          usage 1>&2; exit 1;;
    esac
    shift 2
done

if [[ -z "${image-}" ]]; then
    usage 1>&2
    exit 1
fi

vpc=$IBMCLOUD_VPC_NAME
zone=$IBMCLOUD_VPC_ZONE
subnet=$IBMCLOUD_VPC_SUBNET_NAME
image=${image%.qcow2}

case "$image" in
    *-amd64) profile=bx2-2x8 ;;
    *-s390x) profile=bz2-2x8 ;;
    *)       echo "$0: image for unknown architecture: $image" 1>&2; exit 1 ;;
esac

[ "${SE_BOOT:-0}" = "1" ] && profile=bz2e-2x8

name=$(printf "imagetest-%.8s-%s" "$(uuidgen)" "$image")

export IBMCLOUD_HOME=$(pwd -P)
./login.sh

echo "Create an instance of $image with profle $profile"

id=$(ibmcloud is instance-create "$name" "$vpc" "$zone" "$profile" "$subnet" --image "$image" --output json | jq -r .id)

trap "ibmcloud is instance-delete -f '$id'" 0

echo "Wait for instance $id to become \"running\""

while ! ibmcloud is instance $id --output json | jq -e '.status == "running"' > /dev/null; do
    sleep 5
done

if [[ -n "${SKIP_VERIFY_CONSOLE:-}" ]]; then
    echo "SKIP_VERIFY_CONSOLE is set. Skipping console check..."
    exit 0
fi

echo "Watch console log..."

python3 - "$id" <<'END'
import os
import pty
import sys
from select import select
from time import time

cmd = ["ibmcloud", "is", "instance-console", sys.argv[1]]

pid, fd = pty.fork()
if pid == 0:
    os.execlp(cmd[0], *cmd)

deadline = time() + 600
exit_status = 1
count = 0

while True:
    fds, _, _ = select([fd], [], [], 5)
    if fd in fds:
        try:
            data = os.read(fd, 1024)
        except OSError:
            data = b""
        if not data:
            break
        os.write(1, data)
        if data.find(b" login: ") >= 0:
            count += 1
        if data.find(b"Reached target ") >= 0 and data.find(b"Cloud-init target") >= 0:
            count += 1
        if count >= 3:
            exit_status = 0
            break
    elif time() < deadline:
        os.write(fd, b"\n") # send "return"
    else:
        break

os.write(fd, b"\035\n") # send ^[
os.write(1, b"\n")
sys.exit(exit_status)
END
