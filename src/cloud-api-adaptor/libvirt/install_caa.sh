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

LIBVIRT_USER="${LIBVIRT_USER:-$USER}"
LIBVIRT_NET="${LIBVIRT_NET:-default}"
LIBVIRT_POOL="${LIBVIRT_POOL:-default}"
SSH_KEY_FILE="${SSH_KEY_FILE:-id_rsa}"

# Apply the 'node.kubernetes.io/worker' label on all worker nodes.
#
label_workers() {
	local workers
	local label='node.kubernetes.io/worker'

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
	You must have at least KUBECONFIG exported in the environment.

	Use: $0 [-h|help]

	Use the following environment variables to change the installation
	parameters:
	LIBVIRT_IP    (default determined from LIBVIRT_NET)
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
	local pods
	pods="$(kubectl get pods --no-headers -n "$ns" | \
		awk '{ print $1 }')"
	for pod in $pods; do
		kubectl wait --for=condition=Ready --timeout 240s -n "$ns" "pod/${pod}"
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

	if eval "[ -z ${KUBECONFIG:-} ]"; then
		echo "ERROR: variable 'KUBECONFIG' is not exported"
		usage
		exit 1
	fi

	if eval "[ -z ${LIBVIRT_IP:-} ]"; then
		echo "WARNING: LIBVIRT_IP is not exported. Finding ip from network ($LIBVIRT_NET) XML configuration"
		LIBVIRT_IP=$(virsh -c qemu:///system net-dumpxml $LIBVIRT_NET | sed -n "s/.*ip address='\(.*\)' .*/\1/p")
		if eval "[ -z ${LIBVIRT_IP:-} ]"; then
			echo "ERROR: Static Route not defined in network XML configuration"
			exit 1
		fi
		echo "Using LIBVIRT_IP: $LIBVIRT_IP"
	fi


	label_workers

	local values_file="$script_dir/../install/charts/peerpods/providers/libvirt.yaml"
	if [ ! -f "$values_file" ]; then
		echo "ERROR: kustomization file not found: $values_file"
		exit 1
	fi

	[ "$LIBVIRT_NET" == "default" ] || \
		sed -i -e 's/\(\s\+\)#\s\(LIBVIRT_NET: \).*/\1\2"'"${LIBVIRT_NET}"'"/' \
		"$values_file"
	[ "$LIBVIRT_POOL" == "default" ] || \
		sed -i -e 's/\(\s\+\)#\s\(LIBVIRT_POOL: \).*/\1\2"'"${LIBVIRT_POOL}"'"/' \
		"$values_file"
	[ "$SSH_KEY_FILE" == "default" ] || \
		{
		printf '%s\n' 'providerSecrets:' '  libvirt:' '    id_rsa: |'
		sed 's/^/      /' "${SSH_KEY_FILE}"
		} > "$script_dir/../install/charts/peerpods/providers/libvirt-secrets.yaml"

	local libvirt_uri="qemu+ssh://${LIBVIRT_USER}@${LIBVIRT_IP}/system?no_verify=1"
	sed -i -e 's!\(\s\+\)#\s\(LIBVIRT_URI: \).*!\1\2"'"${libvirt_uri}"'"!' \
		-e 's/libvirt: {}/libvirt:/' \
		"$values_file"

	# Finally install the operator
	(cd "$script_dir/.." && make CLOUD_PROVIDER=libvirt deploy)

	wait_pods
}

main "$@"
