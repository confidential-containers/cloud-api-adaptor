#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

.PHONY: all build check fmt vet clean
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

test:
	go test -v $(GOFLAGS) -cover $(PACKAGES)

check: fmt vet

fmt:
	find $(SOURCEDIRS) -name '*.go' -print0 | xargs -0 gofmt -l -d

vet:
	go vet $(PACKAGES)

clean:
	rm -fr $(BINARIES)
