#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")
bin_dir=${script_dir}/../bin
kata_dir=${script_dir}/../../../../kata-containers
providers=( aws ibmcloud libvirt )

function build_cloud_api_adaptor() {
	pushd "${script_dir}/../../.."
        CLOUD_PROVIDER=$1 make clean
        CLOUD_PROVIDER=$1 make
	cp cloud-api-adaptor ${bin_dir}/cloud-api-adaptor-$1 
	popd
}

function build_kata_runtime() {
	pushd "${kata_dir}/src/runtime"
        make
	cp containerd-shim-kata-v2 kata-monitor kata-runtime ${bin_dir}/ 
	popd
}
function main() {
	mkdir -p ${bin_dir}
	for i in "${providers[@]}"
	do
		build_cloud_api_adaptor $i
	done
	build_kata_runtime
}

main "$@"
