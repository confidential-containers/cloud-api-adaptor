#!/bin/bash
#
# Copyright Confidential Containers Contributors
#
# SPDX-License-Identifier: Apache-2.0
#
# Use this script to run the end-to-end tests locally.
#
set -o errexit
set -o nounset
set -o pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly webhook_dir="$(cd "${script_dir}/../../" && pwd)"

# Whether to run this script in debug mode or not.
debug=0
export IMG="peer-pods-webhook:test"

cleanup () {
	if [ $debug -eq 1 ]; then
		echo "INFO: running in debug mode. Do not clean up the test environment."
		return
	fi

	echo "INFO: clean up the test environment"
	pushd "$webhook_dir" >/dev/null
	make kind-delete || true
	docker rmi "$IMG" || true
}

# Start the cluster and ensure it is ready to use.
#
cluster_up() {
	pushd "$webhook_dir" >/dev/null
	make kind-cluster
	popd >/dev/null

	local cert_manager_ns="cert-manager"
	local cert_manager_pods="$(kubectl get pods -n "$cert_manager_ns" 2>/dev/null | \
		grep cert-manager-webhook |awk '{ print $1}')"

	if [ -z "$cert_manager_pods" ]; then
		echo "ERROR: failed to get the certification manager webhook pods"
		exit 1
	fi

	local pod
	for pod in $cert_manager_pods; do
		kubectl wait --for=condition=Ready --timeout=60s \
			-n "$cert_manager_ns" "pod/$pod"
	done
}

# Install the webhook and ensure it is ready to use.
#
install_webhook() {
	local ns="peer-pods-webhook-system"

	pushd "$webhook_dir" >/dev/null
	make deploy
	popd >/dev/null

	local webhook_pods="$(kubectl get pods -n "$ns" 2>/dev/null | \
		grep peer-pods-webhook-controller-manager |awk '{ print $1}')"

	if [ -z "$webhook_pods" ];then
		echo "ERROR: failed to get the webhook controller manager pods"
		exit 1
	fi

	local pod
	for pod in $webhook_pods; do
		kubectl wait --for=condition=Ready --timeout=60s -n "$ns" \
			"pod/$pod"
	done
}

main() {
	parse_args $@

	for cmd in bats docker kind; do
		if ! command -v "$cmd" &>/dev/null; then
			echo "ERROR: $cmd command is required for this script"
			exit 1
		fi
	done

	trap cleanup EXIT

	echo "INFO: start the test cluster"
	cluster_up

	pushd "$webhook_dir" >/dev/null

	echo "INFO: build the webhook"
	make docker-build

	echo "INFO: load the $IMG image into the cluster"
	make kind-load

	echo "INFO: install the webhook in the cluster"
	install_webhook

	echo "INFO: run tests"
	bats tests/e2e/webhook_tests.bats

	popd >/dev/null
}

parse_args() {
	while getopts "dh" opt; do
		case $opt in
			d) debug=1;;
			h) usage && exit 0;;
			*) usage && exit 1;;
		esac
	done
}

usage() {
	cat <<-EOF
	Start a k8s cluster with kind, build and install the webhook then
	run end-to-end tests.

	It requires bats, docker and kind to run.

	Use: $0 [-d] [-h], where:
	-d: debug mode. It will leave created resources (cluster, image, and etc...)
	-h: show this usage
	EOF
}

main "$@"
