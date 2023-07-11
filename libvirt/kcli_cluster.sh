#!/bin/bash
#
# Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Manage Kubernetes clusters with kcli.
#
set -o errexit
set -o nounset
set -o pipefail

CLUSTER_DISK_SIZE="${CLUSTER_DISK_SIZE:-20}"
CLUSTER_CONTROL_NODES="${CLUSTER_CONTROL_NODES:-1}"
CLUSTER_NAME="${CLUSTER_NAME:-peer-pods}"
CLUSTER_IMAGE="${CLUSTER_IMAGE:-ubuntu2004}"
CLUSTER_WORKERS="${CLUSTER_WORKERS:-1}"
LIBVIRT_NETWORK="${LIBVIRT_NETWORK:-default}"
LIBVIRT_POOL="${LIBVIRT_POOL:-default}"

# Wait until the command return true.
#
# Parameters:
#   $1 - wait time.
#   $2 - sleep time.
#   $3 - the command to run.
#
wait_for_process() {
	local wait_time="$1"
	local sleep_time="$2"
	local cmd="$3"

	while ! eval "$cmd" && [ "$wait_time" -gt 0 ]; do
		sleep "$sleep_time"
		wait_time=$((wait_time-"$sleep_time"))
	done

	[ "$wait_time" -ge 0 ]
}

# Create the cluster.
#
create () {
	kcli create kube generic \
		-P domain="kata.com" \
		-P pool="$LIBVIRT_POOL" \
		-P ctlplanes="$CLUSTER_CONTROL_NODES" \
		-P workers="$CLUSTER_WORKERS" \
		-P network="$LIBVIRT_NETWORK" \
		-P image="$CLUSTER_IMAGE" \
		-P sdn=flannel \
		-P nfs=false \
		-P disk_size="$CLUSTER_DISK_SIZE" \
		"$CLUSTER_NAME"

	export KUBECONFIG=$HOME/.kcli/clusters/$CLUSTER_NAME/auth/kubeconfig

	local cmd="kubectl get nodes | grep '\<Ready\>.*worker'"
	echo "Wait at least one worker be Ready"
	if ! wait_for_process "330" "30" "$cmd"; then
		echo "ERROR: worker nodes not ready."
		kubectl get nodes
		exit 1
	fi

	# Ensure that system pods are running or completed.
	cmd="[ \$(kubectl get pods -A --no-headers | grep -v 'Running\|Completed' | wc -l) -eq 0 ]"
	echo "Wait system pods be running or completed"
	if ! wait_for_process "90" "30" "$cmd"; then
		echo "ERROR: not all pods are Running or Completed."
		kubectl get pods -A
		exit 1
	fi
}

# Delete the cluster.
#
delete () {
	kcli delete -y kube "${CLUSTER_NAME}"
}

usage () {
	cat <<-EOF
	Create/delete a Kubernetes cluster with kcli tool.

	Use: $0 [-h|help] COMMAND
	where COMMAND can be:
	create    Create the cluster. Use the following environment variables
	          to change the creation parameters:
	          CLUSTER_DISK_SIZE       (default "${CLUSTER_DISK_SIZE}")
	          CLUSTER_IMAGE           (default "${CLUSTER_IMAGE}")
	          CLUSTER_CONTROL_NODES   (default "${CLUSTER_CONTROL_NODES}")
	          CLUSTER_NAME            (default "${CLUSTER_NAME}")
	          LIBVIRT_NETWORK         (default "${LIBVIRT_NETWORK}")
	          LIBVIRT_POOL            (default "${LIBVIRT_POOL}")
	          CLUSTER_WORKERS         (default "${CLUSTER_WORKERS}").
	delete    Delete the cluster. Specify the cluster name with
	          CLUSTER_NAME (default "${CLUSTER_NAME}").
	EOF
}

main() {
	if ! command -v kcli >/dev/null; then
		echo "ERROR: kcli command is required. See https://kcli.readthedocs.io/en/latest/#installation"
		exit 1
	fi

	# Parse arguments.
	#
	if [ "$#" -lt 1 ]; then
		usage
		exit 1
	fi
	case "$1" in
		-h|help)
			usage
			exit 0;;
		create) create;;
		delete) delete;;
		*)
			echo "Unknown command: $1"
			usage
			exit 1;;
	esac
}

main "$@"
