#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

.PHONY: all build check fmt vet clean image push deploy
ifndef CLOUD_PROVIDER
$(error CLOUD_PROVIDER is not set)
endif

GOFLAGS    ?= -tags=$(CLOUD_PROVIDER)
BINARIES   := cloud-api-adaptor agent-protocol-forwarder
SOURCEDIRS := ./cmd ./pkg
PACKAGES   := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES    := $(shell find $(SOURCEDIRS) -name '*.go' -print)

IMAGE_BUILD_CMD ?= podman build
IMAGE_PUSH_CMD ?= podman push
IMAGE_REGISTRY ?= quay.io/confidential-containers
IMAGE_NAME := cloud-api-adaptor
IMAGE_TAG_NAME ?= latest
IMAGE_REPO ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME)
IMAGE_TAG ?= $(IMAGE_REPO):$(IMAGE_TAG_NAME)

all: build
build: $(BINARIES)

$(BINARIES): $(SOURCES)
ifeq ($(CLOUD_PROVIDER),libvirt)
	go build $(GOFLAGS) -o "$@" "cmd/$@/main.go"
else
	CGO_ENABLED=0 go build $(GOFLAGS) -o "$@" "cmd/$@/main.go"
endif

# Build the docker image with $IMAGE_TAG
image:
	$(IMAGE_BUILD_CMD) -t $(IMAGE_TAG) .

# Push the docker image to $IMAGE_TAG
push:
	$(IMAGE_PUSH_CMD) $(IMAGE_TAG)

# Deploy cloud-api-adaptor pod, pull image from $IMAGE_TAG if set
deploy:
	kubectl apply -f $(CLOUD_PROVIDER)/deploy/
	cat deploy/cloud-api-adaptor.yaml | sed "s#quay.io/confidential-containers/cloud-api-adaptor:latest#${IMAGE_TAG}#g" | kubectl apply -f -

delete:
	kubectl delete -f $(CLOUD_PROVIDER)/deploy/
	kubectl delete deploy/cloud-api-adaptor.yaml

test:
	# Note: sending stderr to stdout so that tools like go-junit-report can
	# parse build errors.
	go test -v $(GOFLAGS) -cover $(PACKAGES) 2>&1

check: fmt vet

fmt:
	find $(SOURCEDIRS) -name '*.go' -print0 | xargs -0 gofmt -l -d

vet:
	go vet $(PACKAGES)

clean:
	rm -fr $(BINARIES)
