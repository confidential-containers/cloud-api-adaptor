# Build the manager binary
# golang:1.25.11
FROM --platform=$BUILDPLATFORM golang@sha256:dd7d32e19b28621cd982082397fc0510d396805b717d5e77466aa2dd692340de AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY tlsconfig.go tlsconfig.go
#COPY api/ api/
#COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -a -o manager .

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# gcr.io/distroless/static:nonroot
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
