#!/bin/bash
#
# Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Install the CAA operator in a Kubernetes cluster.
#
set -o errexit
set -o nounset
set -o pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

LIBVIRT_IP="${LIBVIRT_IP:-}"
LIBVIRT_USER="${LIBVIRT_USER:-$USER}"
LIBVIRT_NET="${LIBVIRT_NET:-default}"
LIBVIRT_POOL="${LIBVIRT_POOL:-default}"
SSH_KEY_FILE="${SSH_KEY_FILE:-}"

# Apply the 'node-role.kubernetes.io/worker' label on all worker nodes.
#
label_workers() {
	local workers
	local label='node-role.kubernetes.io/worker'

	workers="$(kubectl get nodes --no-headers | grep '\<worker\>' | awk '{ print $1 }')"
	for nodename in $workers; do
		if ! kubectl get "node/${nodename}" -o jsonpath='{.metadata.labels}' | \
			grep "$label" >/dev/null; then
			kubectl label node "$nodename" "${label}="
		fi
	done
}

usage() {
	cat <<-EOF
	Install cloud-api-adaptor in the Kubernetes cluster.
	You must have KUBECONFIG and LIBVIRT_IP exported in the environment.

	Use: $0 [-h|help]

	Use the following environment variables to change the installation
	parameters:
	LIBVIRT_IP    (required)
	LIBVIRT_USER  (default "$LIBVIRT_USER")
	LIBVIRT_NET   (default "$LIBVIRT_NET")
	LIBVIRT_POOL  (default "$LIBVIRT_POOL")
	SSH_KEY_FILE  (default "$SSH_KEY_FILE")
	EOF
}
# Wait all pods on confidential containers namespace be ready.
#
wait_pods() {
	local ns="confidential-containers-system"
	local pods="$(kubectl get pods --no-headers -n "$ns" | \
		awk '{ print $1 }')"
	for pod in $pods; do
		kubectl wait --for=condition=Ready --timeout 120s -n "$ns" "pod/${pod}"
	done
}

main() {
	# Parse arguments.
	#
	if [ -n "${1:-}" ]; then
		case "$1" in
			-h|help) usage && exit 0;;
			*)
				echo "Unknown command: $1"
				usage && exit 1;;
		esac
	fi

	local var
	for var in KUBECONFIG LIBVIRT_IP; do
		if eval "[ -z \${${var}:-} ]"; then
			echo "ERROR: variable '$var' is not exported"
			usage
			exit 1
		fi
	done

	label_workers

	local kustomization_file="$script_dir/../install/overlays/libvirt/kustomization.yaml"
	if [ ! -f "$kustomization_file" ]; then
		echo "ERROR: kustomization file not found: $kustomization_file"
		exit 1
	fi

	[ "$LIBVIRT_NET" == "default" ] || \
		sed -i -e 's/\(\s\+-\sLIBVIRT_NET=\).*/\1"'"${LIBVIRT_NET}"'"/' \
		"$kustomization_file"
	[ "$LIBVIRT_POOL" == "default" ] || \
		sed -i -e 's/\(\s\+-\sLIBVIRT_POOL=\).*/\1"'"${LIBVIRT_POOL}"'"/' \
		"$kustomization_file"
	[ "$SSH_KEY_FILE" == "default" ] || \
		sed -i -e 's@\(\s\+\)#\?- id_rsa.*@\1- '"$SSH_KEY_FILE"'@' \
		"$kustomization_file"

	local libvirt_uri="qemu+ssh://${LIBVIRT_USER}@${LIBVIRT_IP}/system?no_verify=1"
	sed -i -e 's#\(\s\+- LIBVIRT_URI=\).*#\1"'"${libvirt_uri}"'"#' \
		"$kustomization_file"

	# Finally install the operator
	(cd "$script_dir/.." && make CLOUD_PROVIDER=libvirt deploy)

	wait_pods
}

main "$@"
