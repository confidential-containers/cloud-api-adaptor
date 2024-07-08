ARG BUILD_TYPE=dev
ARG BUILDER_BASE=quay.io/confidential-containers/golang-fedora:1.21.12-39
ARG BASE=registry.fedoraproject.org/fedora:39

# This dockerfile uses Go cross-compilation to build the binary,
# we build on the host platform ($BUILDPLATFORM) and then copy the
# binary into the container image of the target platform ($TARGETPLATFORM)
# that was specified with --platform. For more details see:
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide/
FROM --platform=$BUILDPLATFORM $BUILDER_BASE AS builder-release
# For `dev` builds due to CGO constraints we have to emulate the target platform
# instead of using Go's cross-compilation
FROM --platform=$TARGETPLATFORM $BUILDER_BASE AS builder-dev

RUN dnf install -y libvirt-devel && dnf clean all

FROM builder-${BUILD_TYPE} AS builder
ARG RELEASE_BUILD
ARG COMMIT
ARG VERSION
ARG TARGETARCH
ARG YQ_VERSION

RUN go install github.com/mikefarah/yq/v4@$YQ_VERSION

WORKDIR /work
COPY cloud-api-adaptor/go.mod cloud-api-adaptor/go.sum ./cloud-api-adaptor/
COPY cloud-providers ./cloud-providers
COPY peerpod-ctrl ./peerpod-ctrl
WORKDIR /work/cloud-api-adaptor
RUN go mod download
COPY cloud-api-adaptor/entrypoint.sh cloud-api-adaptor/Makefile cloud-api-adaptor/Makefile.defaults cloud-api-adaptor/versions.yaml ./
COPY cloud-api-adaptor/hack  ./hack
COPY cloud-api-adaptor/cmd   ./cmd
COPY cloud-api-adaptor/pkg   ./pkg
COPY cloud-api-adaptor/proto ./proto

RUN CC=gcc make ARCH=$TARGETARCH COMMIT=$COMMIT VERSION=$VERSION RELEASE_BUILD=$RELEASE_BUILD cloud-api-adaptor

FROM --platform=$TARGETPLATFORM $BASE AS base-release

FROM base-release AS base-dev
RUN dnf install -y libvirt-libs /usr/bin/ssh && dnf clean all

FROM base-${BUILD_TYPE}
COPY --from=builder /work/cloud-api-adaptor/cloud-api-adaptor /work/cloud-api-adaptor/entrypoint.sh /usr/local/bin/
CMD ["entrypoint.sh"]
