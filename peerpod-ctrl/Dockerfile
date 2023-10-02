# Build the manager binary
FROM --platform=$TARGETPLATFORM quay.io/confidential-containers/golang-fedora:1.20.8-38 as builder
ARG TARGETOS
ARG TARGETARCH
ARG CGO_ENABLED=1
ARG GOFLAGS

WORKDIR /workspace
RUN if [ "$CGO_ENABLED" = 1 ] ; then dnf install -y libvirt-devel && dnf clean all; fi
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
# CC=gcc because the cgo compiler will always be gcc in image golang-fedora, even for s390x and ppc64le
RUN CC=gcc CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build ${GOFLAGS} -a -o manager main.go

# Target Image
FROM --platform=$TARGETPLATFORM registry.fedoraproject.org/fedora:38
ARG CGO_ENABLED=1

RUN if [ "$CGO_ENABLED" = 1 ] ; then dnf install -y libvirt-libs openssh-clients && dnf clean all; fi
WORKDIR /
COPY --from=builder /workspace/manager .

ENTRYPOINT ["/manager"]
