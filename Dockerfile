# syntax=docker/dockerfile:1.5-labs
ARG BUILD_TYPE=dev
ARG BUILDER_BASE=quay.io/confidential-containers/golang-fedora:1.20.8-36
ARG BASE=registry.fedoraproject.org/fedora:38

FROM --platform=$TARGETPLATFORM $BUILDER_BASE as builder-release

FROM builder-release as builder-dev
RUN dnf install -y libvirt-devel && dnf clean all

FROM builder-${BUILD_TYPE} AS builder
ARG RELEASE_BUILD
ARG COMMIT
ARG VERSION
ARG TARGETARCH
ARG YQ_VERSION
ARG YQ_CHECKSUM

ADD --checksum=${YQ_CHECKSUM} https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_amd64 /usr/local/bin/yq
RUN chmod a+x /usr/local/bin/yq

WORKDIR /work
COPY go.mod go.sum ./
RUN go mod download
COPY entrypoint.sh Makefile Makefile.defaults versions.yaml ./
COPY cmd   ./cmd
COPY pkg   ./pkg
COPY proto ./proto
RUN CC=gcc make ARCH=$TARGETARCH COMMIT=$COMMIT VERSION=$VERSION RELEASE_BUILD=$RELEASE_BUILD cloud-api-adaptor

FROM --platform=$TARGETPLATFORM $BASE as base-release

FROM base-release as base-dev
RUN dnf install -y libvirt-libs /usr/bin/ssh && dnf clean all

FROM base-${BUILD_TYPE}
COPY --from=builder /work/cloud-api-adaptor /work/entrypoint.sh /usr/local/bin/
CMD ["entrypoint.sh"]
