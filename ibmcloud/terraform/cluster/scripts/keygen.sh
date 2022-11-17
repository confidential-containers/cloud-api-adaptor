#!/bin/bash
#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit -o pipefail -o nounset

cd "$(dirname "${BASH_SOURCE[0]}")"

function usage() {
    echo "Usage: $0 --bastion <bastion host> [--ssh-private-key <ssh private key file>] host1 [host2...]"
}

declare -a worker_nodes

while (( $# )); do
    case "$1" in
        --bastion)        bastion=$2 ;;
        --ssh-private-key)    ssh_private_key=$2 ;;
        --help)     usage; exit 0 ;;
        *)          break
    esac
    shift 2
done

hosts=("$@")

if [[ "${#hosts[@]}" -eq 0 || -z "${bastion-}" ]]; then
    usage 1>&2
    exit 1
fi

tmpdir=$(mktemp -d)
ssh_known_hosts="$tmpdir/known_hosts"

function cleanup () {
    rm -fr "$tmpdir"
}

trap cleanup 0

opts=(-o StrictHostKeyChecking=accept-new -o "UserKnownHostsFile=$ssh_known_hosts")
if [[ -n "${ssh_private_key-}" ]]; then
    opts+=( -o "IdentityFile=$ssh_private_key" )
fi

key_type=ed25519
key="$tmpdir/id_$key_type"

ssh "${opts[@]}" -o ConnectionAttempts=300 -o ConnectTimeout=1 -n -l root "$bastion" true

ssh "${opts[@]}" -n -l root "$bastion" bash -c "true; [[ -e '/root/.ssh/id_$key_type' ]] || ssh-keygen -t '$key_type' -N '' -f '/root/.ssh/id_$key_type'"

touch "$key"
chmod 600 "$key"
ssh "${opts[@]}" -n -l root "$bastion" cat "/root/.ssh/id_$key_type"     > "$key"
ssh "${opts[@]}" -n -l root "$bastion" cat "/root/.ssh/id_$key_type.pub" > "$key.pub"

port=22022
ssh_ctl_sock="$tmpdir/ssh-ctl.sock"

for host in "${hosts[@]}"; do
    (
        function stop() {
            ssh "${opts[@]}" -S "$ssh_ctl_sock" -O exit -l root "$bastion" || true
        }
        trap stop 0

        ssh "${opts[@]}" -o ExitOnForwardFailure=yes -S "$ssh_ctl_sock" -L "$port:$host:22" -M -f -N -l root "$bastion"
        [[ $(uname) = "Darwin" ]] && force_option="-f"
        ssh-copy-id ${force_option:-} "${opts[@]}" -i "$key.pub" -p "$port" root@localhost
        
        ssh-keygen -R "[localhost]:$port" -f "$ssh_known_hosts"
    )
done
