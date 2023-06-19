#!/bin/bash

GO_VERSION="1.19.6"
RUST_VERSION="1.66.0"
SKOPEO_VERSION="1.5.0"

# Install dependencies
yum install -y curl protobuf-compiler libseccomp-devel openssl openssl-devel skopeo-${SKOPEO_VERSION}

# Install Golang
curl https://dl.google.com/go/go${GO_VERSION}.linux-ppc64le.tar.gz -o go${GO_VERSION}.linux-ppc64le.tar.gz && \
rm -rf /usr/local/go && tar -C /usr/local -xzf go${GO_VERSION}.linux-ppc64le.tar.gz && \
rm -f go${GO_VERSION}.linux-ppc64le.tar.gz

# Install Rust
curl https://sh.rustup.rs -sSf | sh -s -- -y --default-toolchain ${RUST_VERSION}
rustup target add powerpc64le-unknown-linux-gnu
