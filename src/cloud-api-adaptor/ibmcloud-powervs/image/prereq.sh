#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR=$(dirname "$(realpath "$0")")

# Determine the path to versions.yaml relative to the script's directory
VERSIONS_YAML_PATH=$(realpath "${SCRIPT_DIR}/../../versions.yaml")

GO_VERSION="$(yq '.tools.golang' "${VERSIONS_YAML_PATH}")"
ORAS_VERSION="$(yq '.tools.oras' "${VERSIONS_YAML_PATH}")"

# Install dependencies
yum install -y curl libseccomp-devel openssl openssl-devel skopeo clang clang-devel perl iptables

wget https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_ppc64le.tar.gz
rm -rf /usr/local/bin/oras && tar -C /usr/local/bin -xzf oras_${ORAS_VERSION}_linux_ppc64le.tar.gz && rm -f oras_${ORAS_VERSION}_linux_ppc64le.tar.gz

wget https://rpmfind.net/linux/centos-stream/9-stream/CRB/ppc64le/os/Packages/protobuf-compiler-3.14.0-13.el9.ppc64le.rpm
yum install -y protobuf-compiler-3.14.0-13.el9.ppc64le.rpm

wget https://www.rpmfind.net/linux/centos-stream/9-stream/CRB/ppc64le/os/Packages/device-mapper-devel-1.02.202-2.el9.ppc64le.rpm
yum install -y device-mapper-devel-1.02.202-2.el9.ppc64le.rpm

# Install Golang
curl https://dl.google.com/go/go${GO_VERSION}.linux-ppc64le.tar.gz -o go${GO_VERSION}.linux-ppc64le.tar.gz && \
rm -rf /usr/local/go && tar -C /usr/local -xzf go${GO_VERSION}.linux-ppc64le.tar.gz && \
rm -f go${GO_VERSION}.linux-ppc64le.tar.gz
