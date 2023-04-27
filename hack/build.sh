#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")

registry="${registry:-quay.io/confidential-containers}"
name="cloud-api-adaptor"
release_build=${RELEASE_BUILD:-false}
version=${VERSION:-unknown}
commit=${COMMIT:-unknown}

if [[ "$commit" = unknown ]]; then
	commit=$(git rev-parse HEAD)
	[[ -n "$(git status --porcelain --untracked-files=no)" ]] && commit+='-dirty'
fi

supported_arches=${ARCHES:-"linux/amd64"}

function build_caa_payload() {
	pushd "${script_dir}/.."

	local tag_string="-t ${registry}/${name}:latest -t ${registry}/${name}:dev-${commit}"
	local build_type=dev

	if [[ "$release_build" == "true" ]]; then
		tag_string="-t ${registry}/${name}:${commit}"
		build_type=release
	fi

	docker buildx build --platform "${supported_arches}" \
		--build-arg RELEASE_BUILD="${release_build}" \
		--build-arg BUILD_TYPE="${build_type}" \
		--build-arg VERSION="${version}" \
		--build-arg COMMIT="${commit}" \
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
