#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

.PHONY: all build check fmt vet clean

BINARIES   := cloud-api-adaptor agent-protocol-forwarder
SOURCEDIRS := ./cmd ./pkg
PACKAGES   := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES    := $(shell find $(SOURCEDIRS) -name '*.go' -print)

all: build
build: $(BINARIES)

$(BINARIES): $(SOURCES)
	CGO_ENABLED=0 go build -o "$@" "cmd/$@/main.go"

test:
	 go test -cover $(PACKAGES)

check: fmt vet

fmt:
	find $(SOURCEDIRS) -name '*.go' -print0 | xargs -0 gofmt -l -d

vet:
	go vet $(PACKAGES)

clean:
	rm -fr $(BINARIES)
