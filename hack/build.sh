#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")

registry="${registry:-quay.io/confidential-containers}"
name="cloud-api-adaptor"
commit_id=${1:-$(git rev-parse HEAD)}

supported_arches=${ARCHES:-"linux/amd64"}

function build_caa_payload() {
	pushd "${script_dir}/.."

	local tag_string="-t ${registry}/${name}:latest -t ${registry}/${name}:dev-${commit_id}"

	if [[ "$RELEASE_BUILD" == "true" ]]; then
		tag_string="-t ${registry}/${name}:${commit_id}"
	fi


	docker buildx build --platform "${supported_arches}" \
		--build-arg RELEASE_BUILD="${RELEASE_BUILD}" \
		-f Dockerfile \
		${tag_string} \
		--push \
		.
	popd
}

function main() {
	build_caa_payload
}

main "$@"
