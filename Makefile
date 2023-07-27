# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

.PHONY: all build check fmt vet clean image deploy delete

SHELL = bash -o pipefail

ARCH        ?= $(subst x86_64,amd64,$(shell uname -m))
# Default is dev build. To create release build set RELEASE_BUILD=true
RELEASE_BUILD ?= false
# CLOUD_PROVIDER is used for runtime -- which provider should be run against the binary/code.
CLOUD_PROVIDER ?=
GOOPTIONS   ?= GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0
GOFLAGS     ?=
BINARIES    := cloud-api-adaptor agent-protocol-forwarder
SOURCEDIRS  := ./cmd ./pkg
PACKAGES    := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES     := $(shell find $(SOURCEDIRS) -name '*.go' -print)
# End-to-end tests overall run timeout.
TEST_E2E_TIMEOUT ?= 60m

# BUILTIN_CLOUD_PROVIDERS is used for binary build -- what providers are built in the binaries.
ifeq ($(RELEASE_BUILD),true)
	BUILTIN_CLOUD_PROVIDERS ?= aws azure ibmcloud vsphere
else
	BUILTIN_CLOUD_PROVIDERS ?= aws azure ibmcloud vsphere libvirt
endif

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

# Targets that depend on .gits-commit can use $(shell cat .git-commit) to get a
# git revision string.  They will only be rebuilt if the revision string
# actually changes.
.PHONY: .git-commit.tmp
.git-commit: .git-commit.tmp
	@cmp $< $@ >/dev/null 2>&1 || cp $< $@
.git-commit.tmp:
	@printf "$$(git rev-parse HEAD 2>/dev/null || echo unknown)" >$@
	@test -n "$$(git status --porcelain --untracked-files=no 2> /dev/null)" && echo -dirty >>$@ || true

VERSION ?= $(shell git describe --match "v[0-9]*" --tags 2> /dev/null | sed -E 's/-[0-9]+-g[0-9a-f]+$$/-dev/' || echo unknown)
COMMIT  ?= $(shell cat .git-commit)

GOFLAGS += -ldflags="-X 'github.com/confidential-containers/cloud-api-adaptor/cmd.VERSION=$(VERSION)' \
                     -X 'github.com/confidential-containers/cloud-api-adaptor/cmd.COMMIT=$(COMMIT)'"

# Build tags required to build cloud-api-adaptor are derived from BUILTIN_CLOUD_PROVIDERS.
# When libvirt is specified, CGO_ENABLED is set to 1.
space := $() $()
comma := ,
GOFLAGS += -tags=$(subst $(space),$(comma),$(strip $(BUILTIN_CLOUD_PROVIDERS)))

ifneq (,$(filter libvirt,$(BUILTIN_CLOUD_PROVIDERS)))
cloud-api-adaptor: GOOPTIONS := $(subst CGO_ENABLED=0,CGO_ENABLED=1,$(GOOPTIONS))
endif

$(BINARIES): .git-commit $(SOURCES)
	$(GOOPTIONS) go build $(GOFLAGS) -o "$@" "cmd/$@/main.go"

##@ Development

.PHONY: escapes
escapes: ## golang memory escapes check
	go build $(GOFLAGS) -gcflags="-m -l" ./... 2>&1 | grep "escapes to heap" 1>&2 || true

.PHONY: test
test: ## Run tests.
	# Note: sending stderr to stdout so that tools like go-junit-report can
	# parse build errors.
	go test -v $(GOFLAGS) -cover $(PACKAGES) 2>&1

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests for single provider.
ifneq ($(CLOUD_PROVIDER),)
	go test -v -tags=$(CLOUD_PROVIDER) -timeout $(TEST_E2E_TIMEOUT) -count=1 ./test/e2e
else
	$(error CLOUD_PROVIDER is not set)
endif

.PHONY: check
check: fmt vet golangci-lint shellcheck tidy-check govulncheck ## Run formatters and linters against the code.

.PHONY: fmt
fmt: ## Run go fmt against code.
	find $(SOURCEDIRS) -name '*.go' -print0 | xargs -0 gofmt -l -s -w

.PHONY: vet
vet: ## Run go vet against code.
	go vet $(GOFLAGS) $(PACKAGES)

.PHONY: shellcheck
shellcheck: ## Run shellcheck against shell scripts.
	./hack/shellcheck.sh

.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint against code.
	./hack/golangci-lint.sh

.PHONY: tidy
tidy:
	./hack/go-tidy.sh

.PHONY: tidy-check
tidy-check:
	./hack/go-tidy.sh --check

.PHONY: govulncheck
govulncheck:
	./hack/govulncheck.sh -v

.PHONY: clean
clean: ## Remove binaries.
	rm -fr $(BINARIES) \
		.git-commit .git-commit.tmp

##@ Build

.PHONY: image
image: .git-commit ## Build and push docker image to $registry
	COMMIT=$(COMMIT) VERSION=$(VERSION) hack/build.sh

##@ Deployment

.PHONY: deploy
deploy: ## Deploy cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml file.
ifneq ($(CLOUD_PROVIDER),)
	kubectl apply -k "github.com/confidential-containers/operator/config/default"
	kubectl apply -k "github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods"
	kubectl apply -k install/overlays/$(CLOUD_PROVIDER)
else
	$(error CLOUD_PROVIDER is not set)
endif

.PHONY: delete
delete: ## Delete cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml file.
ifneq ($(CLOUD_PROVIDER),)
	kubectl delete -k install/overlays/$(CLOUD_PROVIDER)
else
	$(error CLOUD_PROVIDER is not set)
endif
