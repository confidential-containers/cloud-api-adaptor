# Build the manager binary
# golang:1.26.10
FROM --platform=$BUILDPLATFORM golang@sha256:fb456410ab4c979617acaf28c46b2b9fe60b11bcc3dc8ceea0eaeb3e7cd0f8a8 AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY webhook/go.mod webhook/go.mod
COPY webhook/go.sum webhook/go.sum
COPY cloud-providers cloud-providers
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
WORKDIR /workspace/webhook
RUN go mod download

# Copy the go source
COPY webhook/main.go main.go
COPY webhook/pkg/ pkg/

# Build
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -a -o manager .

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# gcr.io/distroless/static:nonroot
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39
WORKDIR /
COPY --from=builder /workspace/webhook/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
