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

dev_tags=${DEV_TAGS:-"latest,dev-${commit}"}
release_tags=${RELEASE_TAGS:-"${commit}"}

supported_arches=${ARCHES:-"linux/amd64"}

# Get a list of comma-separated tags (e.g. latest,dev-5d0da3dc9764), return
# the tag string (e.g "-t ${registry}/${name}:latest -t ${registry}/${name}:dev-5d0da3dc9764")
#
function get_tag_string() {
	local tags="$1"
	local tag_string=""

	for tag in ${tags/,/ };do
		tag_string+=" -t ${registry}/${name}:${tag}"
	done

	echo "$tag_string"
}

function build_caa_payload() {
	pushd "${script_dir}/.."

	local tag_string
	local build_type=dev

	tag_string="$(get_tag_string "$dev_tags")"
	if [[ "$release_build" == "true" ]]; then
		tag_string="$(get_tag_string "$release_tags")"
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
