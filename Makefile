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

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

$(BINARIES): $(SOURCES)
ifeq ($(CLOUD_PROVIDER),libvirt)
	go build $(GOFLAGS) -o "$@" "cmd/$@/main.go"
else
	CGO_ENABLED=0 go build $(GOFLAGS) -o "$@" "cmd/$@/main.go"
endif

##@ Development

.PHONY: escapes
escapes: ## golang memeory escapes check
ifeq ($(CLOUD_PROVIDER),libvirt)
	go build $(GOFLAGS) -gcflags="-m -l" -o cloud-api-adaptor cmd/cloud-api-adaptor/main.go | grep "escapes to heap" || true
	go build $(GOFLAGS) -gcflags="-m -l" -o agent-protocol-forwarder cmd/agent-protocol-forwarder/main.go | grep "escapes to heap" || true
else
	CGO_ENABLED=0 go build -gcflags="-m -l" $(GOFLAGS) -o cloud-api-adaptor cmd/cloud-api-adaptor/main.go | grep "escapes to heap" || true
	CGO_ENABLED=0 go build -gcflags="-m -l" $(GOFLAGS) -o agent-protocol-forwarder cmd/agent-protocol-forwarder/main.go | grep "escapes to heap" || true
endif

.PHONY: test
test: ## Run tests.
	# Note: sending stderr to stdout so that tools like go-junit-report can
	# parse build errors.
	go test -v $(GOFLAGS) -cover $(PACKAGES) 2>&1

.PHONY: check
check: fmt vet ## Run go vet and go vet against the code.

.PHONY: fmt
fmt: ## Run go fmt against code.
	find $(SOURCEDIRS) -name '*.go' -print0 | xargs -0 gofmt -l -s -w

.PHONY: vet
vet: ## Run go vet against code.
	go vet $(GOFLAGS) $(PACKAGES)

.PHONY: clean
clean: ## Remove binaries.
	rm -fr $(BINARIES)

##@ Build

.PHONY: image
image: ## Build and push docker image to $registry
	hack/build.sh

##@ Deployment

.PHONY: deploy 
deploy: ## Deploy cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml file.
	kubectl apply -f install/yamls/deploy.yaml
	kubectl apply -k install/overlays/$(CLOUD_PROVIDER)

.PHONY: delete
delete: ## Delete cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml file.
	kubectl delete -k install/overlays/$(CLOUD_PROVIDER)
