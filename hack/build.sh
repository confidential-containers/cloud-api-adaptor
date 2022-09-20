#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")

registry="${registry:-quay.io/confidential-containers/cloud-api-adaptor-${CLOUD_PROVIDER}}"

supported_arches=(
	"linux/amd64"
)

function setup_env_for_arch() {
	case "$1" in
		"linux/amd64")
			kernel_arch="x86_64"
			;;
		(*) echo "$1 is not supported" && exit 1
	esac
}

function build_caa_payload() {
	pushd "${script_dir}/.."

	tag=$(date +%Y%m%d%H%M%s)

	for arch in ${supported_arches[@]}; do
		setup_env_for_arch "${arch}"

		echo "Building cloud-api-adaptor ${CLOUD_PROVIDER} image for ${arch}"
		docker buildx build \
			--build-arg ARCH="${kernel_arch}" \
			--build-arg CLOUD_PROVIDER="${CLOUD_PROVIDER}" \
			-f Dockerfile \
			-t "${registry}:${kernel_arch}-${tag}" \
			--platform="${arch}" \
			--load \
			.
		docker push "${registry}:${kernel_arch}-${tag}"
	done

	docker manifest create \
		${registry}:${tag} \
		--amend ${registry}:x86_64-${tag}

	docker manifest create \
		${registry}:latest \
		--amend ${registry}:x86_64-${tag}

	docker manifest push ${registry}:${tag}
	docker manifest push --purge ${registry}:latest

	popd
}

function main() {
	build_caa_payload
	# TODO: kustomize with the tag
}

main "$@"
