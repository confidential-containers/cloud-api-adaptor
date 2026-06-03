# Build the manager binary
# golang:1.25.11
FROM --platform=$TARGETPLATFORM golang@sha256:dd7d32e19b28621cd982082397fc0510d396805b717d5e77466aa2dd692340de AS builder
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
# debian:trixie-slim (matches golang:1.25.11 base for library compatibility)
FROM --platform=$TARGETPLATFORM debian@sha256:b6e2a152f22a40ff69d92cb397223c906017e1391a73c952b588e51af8883bf8
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
