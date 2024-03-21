#!/usr/bin/env bash
#
# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

# Update the golang-fedora Containerfile with the latest Go version.

set -eou pipefail

if [ $# -ne 1 ]; then
    echo "Usage: $0 <version>"
    exit 1
fi

version=$1

archs=(amd64 arm64 ppc64le s390x)

sed -i 's/^ARG GO_VERSION=.*/ARG GO_VERSION='"${version}"'/' Dockerfile.golang

for arch in "${archs[@]}"; do
    wget "https://go.dev/dl/go${version}.linux-${arch}.tar.gz"
    shasum=$(sha256sum "go${version}.linux-${arch}.tar.gz" | cut -d' ' -f1)
    archUpper=$(echo "${arch}" | tr '[:lower:]' '[:upper:]')
    sed -i 's/^ARG GO_LINUX_'"${archUpper}"'_SHA256=.*/ARG GO_LINUX_'"${archUpper}"'_SHA256='"${shasum}"'/' Dockerfile.golang
    rm "go${version}.linux-${arch}.tar.gz"
done
