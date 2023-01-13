#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")
bin_dir=${script_dir}/../bin
kata_dir=${script_dir}/../../../../kata-containers

supported_go_arches=(
	"x86_64"
	"s390x"
)

function build_kata_shim_v2() {
	pushd "${kata_dir}/src/runtime/cmd/containerd-shim-kata-v2"
		for go_arch in ${supported_go_arches[@]}; do
			env GOOS=linux GOARCH="${go_arch/x86_64/amd64}" CGO_ENABLED=0 go build -o containerd-shim-kata-v2-${go_arch}
			cp containerd-shim-kata-v2-${go_arch} ${bin_dir}/
		done
	popd
}
function main() {
	mkdir -p ${bin_dir}
	build_kata_shim_v2
}

main "$@"
