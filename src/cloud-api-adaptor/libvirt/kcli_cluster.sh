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

CLUSTER_DISK_SIZE="${CLUSTER_DISK_SIZE:-30}"
CLUSTER_CONTROL_NODES="${CLUSTER_CONTROL_NODES:-1}"
CLUSTER_NAME="${CLUSTER_NAME:-peer-pods}"
CLUSTER_IMAGE="${CLUSTER_IMAGE:-ubuntu2204}"
CLUSTER_VERSION="${CLUSTER_VERSION:-1.30.0}"
CLUSTER_WORKERS="${CLUSTER_WORKERS:-1}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-containerd}" # Either "containerd" or "crio"
LIBVIRT_NETWORK="${LIBVIRT_NETWORK:-default}"
LIBVIRT_POOL="${LIBVIRT_POOL:-default}"

ARCH=$(uname -m)
TARGET_ARCH=${ARCH/x86_64/amd64}

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
	local sdn="flannel"

	parameters="-P domain=kata.com \
		-P pool=$LIBVIRT_POOL \
		-P ctlplanes=$CLUSTER_CONTROL_NODES \
		-P workers=$CLUSTER_WORKERS \
		-P network=$LIBVIRT_NETWORK \
		-P image=$CLUSTER_IMAGE \
		-P sdn=$sdn \
		-P nfs=false \
		-P disk_size=$CLUSTER_DISK_SIZE \
		-P version=$CLUSTER_VERSION \
		-P engine=$CONTAINER_RUNTIME"
	# The autolabeller and multus images do not support s390x arch yet
	# disable them for s390x cluster
	if [[ ${TARGET_ARCH} == "s390x" ]]; then
		parameters="$parameters \
			-P arch=$ARCH \
			-P multus=false \
			-P autolabeller=false "
	elif [[ ${TARGET_ARCH} == "aarch64" ]]; then
		parameters="$parameters \
			-P arch=$ARCH \
			-P machine=virt"
	fi
	echo "Download $CLUSTER_IMAGE ${TARGET_ARCH} image"
	# kcli support download image with archs: 'x86_64', 'aarch64', 'ppc64le', 's390x'
	kcli download image $CLUSTER_IMAGE -P arch=${ARCH}

	kcli create kube generic $parameters "$CLUSTER_NAME"

	export KUBECONFIG=$HOME/.kcli/clusters/$CLUSTER_NAME/auth/kubeconfig

	# The autolabeller docker image do not support s390x arch yet
	# use node name to wait one worker node in 'Ready' status and then label worker nodes
	local cmd="kubectl get nodes --no-headers | grep 'worker-.* Ready'"
	echo "Wait at least one worker be Ready"
	if ! wait_for_process "330" "30" "$cmd"; then
		echo "ERROR: worker nodes not ready."
		kubectl get nodes
		exit 1
	fi
	workers=$(kubectl get nodes -o name --no-headers | grep 'worker')
	for worker in $workers; do
		kubectl label --overwrite "$worker" node.kubernetes.io/worker=
		kubectl label --overwrite "$worker" node-role.kubernetes.io/worker=
	done

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
	          CLUSTER_VERSION         (default "${CLUSTER_VERSION}")
	          LIBVIRT_NETWORK         (default "${LIBVIRT_NETWORK}")
	          LIBVIRT_POOL            (default "${LIBVIRT_POOL}")
	          CLUSTER_WORKERS         (default "${CLUSTER_WORKERS}")
	          CONTAINER_RUNTIME       (default "${CONTAINER_RUNTIME}")
	delete    Delete the cluster. Specify the cluster name with
	          CLUSTER_NAME (default "${CLUSTER_NAME}").
	EOF
}

main() {
	# It should use kcli version newer than the build of 2023/11/24
	# that contains the fix to https://github.com/karmab/kcli/pull/623
	local kcli_version
	local kcli_version_min="99.0"
	local kcli_build_date
	local kcli_build_date_min="2023/11/24"

	if ! command -v kcli >/dev/null; then
		echo "ERROR: kcli command is required. See https://kcli.readthedocs.io/en/latest/#installation"
		exit 1
	fi

	kcli_version="$(kcli version | awk '{ print $2}')"
	if [ "${kcli_version/.*/}" -lt "${kcli_version_min/.*/}" ];then
		echo "ERROR: kcli version >= ${kcli_version_min} is required"
		exit 1
	elif [ "${kcli_version}" = "${kcli_version_min}" ]; then
		kcli_build_date="$(kcli version | awk '{ print $5}')"
		if [[ "$kcli_build_date" < "$kcli_build_date_min" ]]; then
			echo "ERROR: kcli ${kcli_version} built since ${kcli_build_date_min} is required"
			exit 1
		fi
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
