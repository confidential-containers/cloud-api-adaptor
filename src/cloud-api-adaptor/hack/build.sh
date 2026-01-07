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

tags=""
if [[ "${release_build}" == "true" ]]; then
	tags=${RELEASE_TAGS:-"${commit}"}
else
	tags=${DEV_TAGS:?}
fi

supported_arches=${ARCHES:-"linux/amd64"}

arch_file_prefix="${script_dir}/../tags-architectures-"
arch_prefix="linux/"

# Get a list of comma-separated tags (e.g. latest,5d0da3dc9764-dev), return
# the tag string (e.g "-t ${registry}/${name}:latest -t ${registry}/${name}:5d0da3dc9764-dev")
#
function get_tag_string() {
	local tags="$1"
	local arch="$2"
	local tag_string=""
	local arch_suffix=""

	# if we are building for a single arch, then also append this to the tag
	if [[ -n "${arch:-}" ]] && [[ "${arch:-}" != *,* ]];then
		# strip the `linux/` from the docker buildx platform format
		arch_suffix="-${arch#"linux/"}"
	fi

	for tag in ${tags/,/ };do
		tag_string+=" -t ${registry}/${name}:${tag}${arch_suffix}"
	done

	echo "$tag_string"
}

function build_caa_payload_image() {
	pushd "${script_dir}/../../"

	local tag_string
	local build_type=dev

	tag_string="$(get_tag_string "$tags" "${supported_arches}")"
	if [[ "${release_build}" == "true" ]]; then
		build_type=release
	fi

	docker buildx build --platform "${supported_arches}" \
		--build-arg RELEASE_BUILD="${release_build}" \
		--build-arg BUILD_TYPE="${build_type}" \
		--build-arg VERSION="${version}" \
		--build-arg COMMIT="${commit}" \
		--build-arg YQ_VERSION="${YQ_VERSION}" \
		-f cloud-api-adaptor/Dockerfile \
		${tag_string} \
		--push \
		.
	popd
}

function get_arch_specific_tag_string() {
	local tags="$1"
	local arch="$2"
	local tag_string=""

	for tag in ${tags/,/ };do
		tag_string+=" -t ${registry}/${name}:${tag}-${arch}"
	done

	echo "$tag_string"
}

# accept one arch as --platform
function build_caa_payload_arch_specific() {
	pushd "${script_dir}/../../"

	arch=${supported_arches#"$arch_prefix"}

	echo "arch="$arch >> "$arch_file_prefix$arch"

	local tag_string
	local build_type=dev

	tag_string="$(get_arch_specific_tag_string "$dev_tags" "${arch}")"
	if [[ "$release_build" == "true" ]]; then
		tag_string="$(get_arch_specific_tag_string "$release_tags" "${arch}")"
		build_type=release
	fi

	docker buildx build --platform "${supported_arches}" \
		--build-arg RELEASE_BUILD="${release_build}" \
		--build-arg BUILD_TYPE="${build_type}" \
		--build-arg VERSION="${version}" \
		--build-arg COMMIT="${commit}" \
		--build-arg YQ_VERSION="${YQ_VERSION}" \
		-f cloud-api-adaptor/Dockerfile \
		${tag_string} \
		--push \
		.
	popd
}

# Get the options
while getopts ":ai" option; do
    case $option in
        a) # image arch specific
            build_caa_payload_arch_specific
            exit;;
        i) # image
            build_caa_payload_image
            exit;;
        \?) # Invalid option
            echo "Error: Invalid option"
   esac
done
