#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")
bin_dir=${script_dir}/../bin
kata_dir=${script_dir}/../../../../kata-containers

function build_kata_runtime() {
	pushd "${kata_dir}/src/runtime"
        make
	cp containerd-shim-kata-v2 ${bin_dir}/ 
	popd
}
function main() {
	mkdir -p ${bin_dir}
	build_kata_runtime
}

main "$@"
