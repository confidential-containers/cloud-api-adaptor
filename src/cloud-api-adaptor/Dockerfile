ARG BUILD_TYPE=dev
# golang:1.25.11 (based on Debian 13 trixie)
ARG BUILDER_BASE=golang@sha256:dd7d32e19b28621cd982082397fc0510d396805b717d5e77466aa2dd692340de
# debian:trixie-slim (matches golang:1.25.11 base for library compatibility)
# Multi-arch manifest list digest supporting amd64, arm64, ppc64le, s390x
ARG BASE=debian@sha256:b6e2a152f22a40ff69d92cb397223c906017e1391a73c952b588e51af8883bf8

# This dockerfile uses Go cross-compilation to build the binary,
# we build on the host platform ($BUILDPLATFORM) and then copy the
# binary into the container image of the target platform ($TARGETPLATFORM)
# that was specified with --platform. For more details see:
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide/
FROM --platform=$BUILDPLATFORM $BUILDER_BASE AS builder-release
ARG YQ_VERSION
RUN go install github.com/mikefarah/yq/v4@$YQ_VERSION
RUN apt-get update && apt-get install -y git && apt-get clean && rm -rf /var/lib/apt/lists/*

# For `dev` builds due to CGO constraints we have to emulate the target platform
# instead of using Go's cross-compilation
FROM $BUILDER_BASE AS builder-dev
ARG YQ_VERSION
RUN apt-get update && \
    apt-get install -y libvirt-dev pkg-config binutils-gold && \
    rm -rf /var/lib/apt/lists/*
RUN go install github.com/mikefarah/yq/v4@$YQ_VERSION

FROM builder-${BUILD_TYPE} AS builder
ARG RELEASE_BUILD
ARG COMMIT
ARG VERSION
ARG TARGETARCH

WORKDIR /work
COPY cloud-api-adaptor/go.mod cloud-api-adaptor/go.sum ./cloud-api-adaptor/
COPY cloud-providers ./cloud-providers
COPY peerpod-ctrl ./peerpod-ctrl
WORKDIR /work/cloud-api-adaptor
RUN go mod download
COPY cloud-api-adaptor/entrypoint.sh cloud-api-adaptor/Makefile cloud-api-adaptor/Makefile.defaults cloud-api-adaptor/versions.yaml ./
COPY cloud-api-adaptor/cmd   ./cmd
COPY cloud-api-adaptor/pkg   ./pkg
COPY cloud-api-adaptor/proto ./proto

RUN CC=gcc ARCH=$TARGETARCH make COMMIT=$COMMIT VERSION=$VERSION RELEASE_BUILD=$RELEASE_BUILD cloud-api-adaptor

FROM builder-release AS iptables

ARG TARGETARCH

COPY cloud-api-adaptor/versions.yaml /versions.yaml

RUN echo "Building iptables-wrapper... " \
    && git clone https://github.com/kubernetes-sigs/iptables-wrappers.git /iptables \
    && cd /iptables \
    && version=$(grep 'iptables-wrapper:' /versions.yaml | awk '{print $2}') \
    && git checkout $version \
    && GOARCH=$TARGETARCH make BIN_DIR=. build

FROM $BASE AS base-release

RUN apt-get update && \
    apt-get install -y ca-certificates iptables && \
    rm -rf /var/lib/apt/lists/*
RUN --mount=type=cache,target=/iptables,from=iptables,source=/iptables,readonly \
    cp /iptables/iptables-wrapper /usr/sbin \
    && /usr/sbin/iptables-wrapper install

FROM base-release AS base-dev
RUN apt-get update && \
    apt-get install -y libvirt0 openssh-client && \
    rm -rf /var/lib/apt/lists/*

FROM base-${BUILD_TYPE}
COPY --from=builder /work/cloud-api-adaptor/cloud-api-adaptor /work/cloud-api-adaptor/entrypoint.sh /usr/local/bin/
CMD ["entrypoint.sh"]
