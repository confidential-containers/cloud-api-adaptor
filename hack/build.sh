#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")

registry="${registry:-quay.io/confidential-containers}"
name="cloud-api-adaptor-${CLOUD_PROVIDER}"
tag=$(date +%Y%m%d%H%M%s)

supported_arches=${ARCHES:-"linux/amd64,linux/s390x"}

function build_caa_payload() {
	pushd "${script_dir}/.."

	docker buildx build --platform "${supported_arches}" \
		--build-arg CLOUD_PROVIDER="${CLOUD_PROVIDER}" \
		-f Dockerfile \
		-t "${registry}/${name}:${tag}" \
		--push \
		.

	popd
}

function main() {
	build_caa_payload
}

main "$@"
