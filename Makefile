#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

.PHONY: all build check fmt vet clean image deploy delete
ifndef CLOUD_PROVIDER
$(error CLOUD_PROVIDER is not set)
endif

GOFLAGS    ?= -tags=$(CLOUD_PROVIDER)
BINARIES   := cloud-api-adaptor agent-protocol-forwarder
SOURCEDIRS := ./cmd ./pkg
PACKAGES   := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES    := $(shell find $(SOURCEDIRS) -name '*.go' -print)

all: build
build: $(BINARIES)

$(BINARIES): $(SOURCES)
ifeq ($(CLOUD_PROVIDER),libvirt)
	go build $(GOFLAGS) -o "$@" "cmd/$@/main.go"
else
	CGO_ENABLED=0 go build $(GOFLAGS) -o "$@" "cmd/$@/main.go"
endif

# Build and push docker image to $regestry
image:
	hack/build.sh

# Deploy cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml
deploy:
	kubectl apply -f install/yamls/deploy.yaml
	kubectl apply -k install/overlays/$(CLOUD_PROVIDER)

delete:
	kubectl delete -k install/overlays/$(CLOUD_PROVIDER)

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
