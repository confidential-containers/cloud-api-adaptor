#!/bin/bash

set -o errexit -o pipefail -o nounset -o errtrace

package=github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper
group=peerpodvolume
version=v1alpha1

basedir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." &>/dev/null && pwd -P)

if [[ ! -d "$basedir/vendor/k8s.io/code-generator" ]]; then
	echo "Run `go mod vendor` before running this script"
	exit 1
fi

cleanup() {
	trap - SIGINT SIGTERM ERR EXIT
	rm -fr "$tmpdir"
}

tmpdir=$(mktemp -d /tmp/gopath.XXXXX)
trap cleanup SIGINT SIGTERM ERR EXIT

mkdir -p "$(dirname "$tmpdir/src/$package")"
ln -s "$basedir" "$tmpdir/src/$package"

bash "$basedir/vendor/k8s.io/code-generator/generate-groups.sh" all \
	"$package/pkg/generated/peerpodvolume" \
	"$package/pkg/apis" \
	"$group:$version" \
	--output-base "$tmpdir/src" \
	--go-header-file /dev/null
