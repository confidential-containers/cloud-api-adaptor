# Build the manager binary
# golang:1.25.10
FROM --platform=$TARGETPLATFORM golang@sha256:3965b9511a9bed199f0b4eb146f99c31971a5c0adb3240d9f25e233cbd9e01c8 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG CGO_ENABLED=1
ARG GOFLAGS

WORKDIR /work
RUN if [ "$CGO_ENABLED" = 1 ] ; then \
    apt-get update && \
    apt-get install -y libvirt-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*; \
    fi
# Copy the Go Modules manifests
COPY peerpod-ctrl/go.mod peerpod-ctrl/go.mod
COPY peerpod-ctrl/go.sum peerpod-ctrl/go.sum
COPY cloud-providers cloud-providers
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
WORKDIR /work/peerpod-ctrl/
RUN go mod download

# Copy the go source
COPY peerpod-ctrl/main.go main.go
COPY peerpod-ctrl/api/ api/
COPY peerpod-ctrl/controllers/ controllers/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
# CC=gcc to ensure consistent CGO compiler across all architectures
RUN CC=gcc CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build ${GOFLAGS} -a -o manager main.go

# Target Image
# debian:trixie-slim (matches golang:1.25.10 base for library compatibility)
FROM --platform=$TARGETPLATFORM debian@sha256:109e2c65005bf160609e4ba6acf7783752f8502ad218e298253428690b9eaa4b
ARG CGO_ENABLED=1

RUN if [ "$CGO_ENABLED" = 1 ] ; then \
    apt-get update && \
    apt-get install -y libvirt0 openssh-client && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*; \
    fi
WORKDIR /
COPY --from=builder /work/peerpod-ctrl/manager .

ENTRYPOINT ["/manager"]
