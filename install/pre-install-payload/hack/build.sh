#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")

registry="${registry:-quay.io/confidential-containers}"
name="peer-pods-pre-install-payload"
tag=$(date +%Y%m%d%H%M%s)

supported_arches="linux/amd64,linux/s390x"

function build_preinstall_payload() {
	pushd "${script_dir}/.."

	docker buildx build --platform ${supported_arches} \
		-f Dockerfile \
		-t "${registry}/${name}:${tag}" \
		--push \
		.

	popd
}

function main() {
	build_preinstall_payload
}

main "$@"
