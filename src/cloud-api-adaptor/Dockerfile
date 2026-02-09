ARG BUILD_TYPE=dev
ARG BUILDER_BASE=quay.io/confidential-containers/golang-fedora:1.24.13-41
ARG BASE=registry.fedoraproject.org/fedora:41

# This dockerfile uses Go cross-compilation to build the binary,
# we build on the host platform ($BUILDPLATFORM) and then copy the
# binary into the container image of the target platform ($TARGETPLATFORM)
# that was specified with --platform. For more details see:
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide/
FROM --platform=$BUILDPLATFORM $BUILDER_BASE AS builder-release
ARG YQ_VERSION
RUN go install github.com/mikefarah/yq/v4@$YQ_VERSION

# For `dev` builds due to CGO constraints we have to emulate the target platform
# instead of using Go's cross-compilation
FROM --platform=$TARGETPLATFORM $BUILDER_BASE AS builder-dev
ARG YQ_VERSION
RUN go install github.com/mikefarah/yq/v4@$YQ_VERSION
RUN dnf install -y libvirt-devel binutils-gold && dnf clean all

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

WORKDIR /iptables
RUN --mount=type=bind,target=/versions.yaml,source=cloud-api-adaptor/versions.yaml,readonly \
    version=$(yq -r .tools.iptables-wrapper /versions.yaml) && \
    GOARCH=$TARGETARCH go install "github.com/kubernetes-sigs/iptables-wrappers@$version" && \
    shopt -s globstar && \
    cp /go/bin/**/iptables-wrappers ./iptables-wrapper && \
    curl -L -o iptables-wrapper-installer.sh "https://raw.githubusercontent.com/kubernetes-sigs/iptables-wrappers/${version#v*-*-}/iptables-wrapper-installer.sh" && \
    chmod 755 iptables-wrapper-installer.sh

FROM --platform=$TARGETPLATFORM $BASE AS base-release

RUN dnf install -y iptables iptables-legacy iptables-nft nftables && dnf clean all
RUN --mount=type=cache,target=/iptables,from=iptables,source=/iptables,readonly \
    cd /iptables && ./iptables-wrapper-installer.sh --no-sanity-check --no-cleanup

FROM base-release AS base-dev
RUN dnf install -y libvirt-libs /usr/bin/ssh && dnf clean all

FROM base-${BUILD_TYPE}
COPY --from=builder /work/cloud-api-adaptor/cloud-api-adaptor /work/cloud-api-adaptor/entrypoint.sh /usr/local/bin/
CMD ["entrypoint.sh"]
