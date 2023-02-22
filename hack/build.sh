#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")

registry="${registry:-quay.io/confidential-containers}"
name="cloud-api-adaptor"
tag=$(date +%Y%m%d%H%M%s)

supported_arches=${ARCHES:-"linux/amd64"}

function build_caa_payload() {
	pushd "${script_dir}/.."

	docker buildx build --platform "${supported_arches}" \
		--build-arg RELEASE_BUILD="${RELEASE_BUILD}" \
		-f Dockerfile \
		-t "${registry}/${name}:${tag}" \
		-t "${registry}/${name}:latest" \
		--push \
		.

	popd
}

function main() {
	build_caa_payload
}

main "$@"
