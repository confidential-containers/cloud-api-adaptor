#!/bin/bash

set -o errexit -o pipefail -o nounset

PODMVINFO_PATH="proto/podvminfo"

protoc \
    --proto_path=$PODMVINFO_PATH \
    --go_out=$GOPATH/src \
    --go-ttrpc_out=$GOPATH/src \
    $PODMVINFO_PATH/podvminfo.proto
