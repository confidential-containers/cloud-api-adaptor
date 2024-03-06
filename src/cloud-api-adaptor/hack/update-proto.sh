#!/bin/bash

set -o errexit -o pipefail -o nounset

basedir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." &>/dev/null && pwd -P)

cd "$basedir/proto"

protoc  --gogottrpc_out=. \
        --gogottrpc_opt=plugins=ttrpc+fieldpath,paths=source_relative \
        podvminfo/podvminfo.proto
