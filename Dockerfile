ARG BUILD_TYPE=dev
ARG BUILDER_BASE=quay.io/confidential-containers/golang-fedora:1.20.12-38
ARG BASE=registry.fedoraproject.org/fedora:38

# This dockerfile uses Go cross-compilation to build the binary,
# we build on the host platform ($BUILDPLATFORM) and then copy the
# binary into the container image of the target platform ($TARGETPLATFORM)
# that was specified with --platform. For more details see:
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide/
FROM --platform=$BUILDPLATFORM $BUILDER_BASE as builder-release
# For `dev` builds due to CGO constraints we have to emulate the target platform
# instead of using Go's cross-compilation
FROM --platform=$TARGETPLATFORM $BUILDER_BASE as builder-dev

RUN dnf install -y libvirt-devel && dnf clean all

FROM builder-${BUILD_TYPE} AS builder
ARG RELEASE_BUILD
ARG COMMIT
ARG VERSION
ARG TARGETARCH
ARG YQ_VERSION

RUN go install github.com/mikefarah/yq/v4@$YQ_VERSION

WORKDIR /work
COPY go.mod go.sum ./
RUN go mod download
COPY entrypoint.sh Makefile Makefile.defaults versions.yaml ./
COPY cmd   ./cmd
COPY pkg   ./pkg
COPY proto ./proto
RUN CC=gcc make ARCH=$TARGETARCH COMMIT=$COMMIT VERSION=$VERSION RELEASE_BUILD=$RELEASE_BUILD cloud-api-adaptor providers

FROM --platform=$TARGETPLATFORM $BASE as base-release

FROM base-release as base-dev
RUN dnf install -y libvirt-libs /usr/bin/ssh && dnf clean all

FROM base-${BUILD_TYPE}
COPY --from=builder /work/cloud-api-adaptor /work/entrypoint.sh /usr/local/bin/
COPY --from=builder /work/*.so /providers/
CMD ["entrypoint.sh"]
