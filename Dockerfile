ARG BUILD_TYPE=dev
ARG GOLANG=golang:1.19
ARG BASE=fedora:36 

FROM --platform=$BUILDPLATFORM $GOLANG AS builder
ARG RELEASE_BUILD
ARG COMMIT
ARG VERSION
ARG TARGETARCH
# Install additional packages required to build libvirt provider
# Need to use the [] syntax as default shell is /bin/sh
RUN if [ "$RELEASE_BUILD" != true ]; then apt-get update -y && apt-get install -y libvirt-dev; fi
WORKDIR /work
COPY go.mod go.sum ./
RUN go mod download
COPY entrypoint.sh Makefile ./
COPY cmd   ./cmd
COPY pkg   ./pkg
COPY proto ./proto
RUN make ARCH=$TARGETARCH COMMIT=$COMMIT VERSION=$VERSION RELEASE_BUILD=$RELEASE_BUILD cloud-api-adaptor

FROM --platform=$TARGETPLATFORM $BASE as base-release

FROM base-release as base-dev
RUN dnf install -y libvirt-libs genisoimage /usr/bin/ssh && dnf clean all

FROM base-${BUILD_TYPE}
COPY --from=builder /work/cloud-api-adaptor /work/entrypoint.sh /usr/local/bin/
CMD ["entrypoint.sh"]
