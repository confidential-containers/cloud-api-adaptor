# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

include Makefile.defaults

.PHONY: all build check fmt vet clean image deploy delete

SHELL = bash -o pipefail

ARCH        ?= $(subst x86_64,amd64,$(shell uname -m))
# Default is dev build. To create release build set RELEASE_BUILD=true
RELEASE_BUILD ?= false
# CLOUD_PROVIDER is used for runtime -- which provider should be run against the binary/code.
CLOUD_PROVIDER ?=
GOOPTIONS   ?= GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0
GOFLAGS     ?=
BINARIES    := cloud-api-adaptor agent-protocol-forwarder process-user-data
SOURCEDIRS  := ./src/cloud-api-adaptor/cmd ./src/cloud-api-adaptor/pkg
PACKAGES    := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES     := $(shell find $(SOURCEDIRS) -name '*.go' -print)
# End-to-end tests overall run timeout.
TEST_E2E_TIMEOUT ?= 60m

RESOURCE_CTRL ?= true

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
	$(GOOPTIONS) go build $(GOFLAGS) -o "$@" ./cmd/$@

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

 ## Run formatters and linters against the code.
.PHONY: check
check: fmt vet golangci-lint shellcheck tidy-check govulncheck packer-check terraform-check

.PHONY: fmt
fmt: ## Run go fmt against code.
	find $(SOURCEDIRS) -name '*.go' -print0 | xargs -0 gofmt -l -s -w

.PHONY: vet
vet: ## Run go vet against code.
	go vet $(GOFLAGS) $(PACKAGES)

.PHONY: shellcheck
shellcheck: ## Run shellcheck against shell scripts.
	./src/cloud-api-adaptor/hack/shellcheck.sh

.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint against code.
	./src/cloud-api-adaptor/hack/golangci-lint.sh

.PHONY: tidy
tidy:
	./src/cloud-api-adaptor/hack/go-tidy.sh

.PHONY: tidy-check
tidy-check:
	./src/cloud-api-adaptor/hack/go-tidy.sh --check

.PHONY: govulncheck
govulncheck:
	./src/cloud-api-adaptor/hack/govulncheck.sh -v

.PHONY: packer-format
packer-format:
	./src/cloud-api-adaptor/hack/packer-check.sh

.PHONY: packer-check
packer-check:
	./src/cloud-api-adaptor/hack/packer-check.sh --check

.PHONY: terraform-format
terraform-format:
	./src/cloud-api-adaptor/hack/terraform-check.sh

.PHONY: terraform-check
terraform-check:
	./src/cloud-api-adaptor/hack/terraform-check.sh --check

.PHONY: clean
clean: ## Remove binaries.
	rm -fr $(BINARIES) \
		.git-commit .git-commit.tmp

##@ Build

.PHONY: image
image: .git-commit ## Build and push docker image to $registry
	COMMIT=$(COMMIT) VERSION=$(VERSION) YQ_VERSION=$(YQ_VERSION) YQ_CHECKSUM=$(YQ_CHECKSUM) hack/build.sh -i

.PHONY: image-with-arch
image-with-arch: .git-commit ## Build the per arch image
	COMMIT=$(COMMIT) VERSION=$(VERSION) YQ_VERSION=$(YQ_VERSION) YQ_CHECKSUM=$(YQ_CHECKSUM) hack/build.sh -a

##@ Deployment

.PHONY: deploy
deploy: ## Deploy cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml file.
ifneq ($(CLOUD_PROVIDER),)
	kubectl apply -k "github.com/confidential-containers/operator/config/release?ref=v0.8.0"
	kubectl apply -k "github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods?ref=v0.8.0"
	kubectl apply -k install/overlays/$(CLOUD_PROVIDER)
else
	$(error CLOUD_PROVIDER is not set)
endif
ifeq ($(RESOURCE_CTRL),true)
	$(MAKE) -C ./src/peerpod-ctrl deploy
endif

.PHONY: delete
delete: ## Delete cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml file.
ifeq ($(RESOURCE_CTRL),true)
	$(MAKE) -C ./src/peerpod-ctrl undeploy
endif
ifneq ($(CLOUD_PROVIDER),)
	kubectl delete -k install/overlays/$(CLOUD_PROVIDER)
else
	$(error CLOUD_PROVIDER is not set)
endif

### PODVM IMAGE BUILDING ###

REGISTRY ?= quay.io/confidential-containers

PODVM_DISTRO ?= ubuntu
PODVM_TAG ?= $(VERSIONS_HASH)

PODVM_BUILDER_IMAGE ?= $(REGISTRY)/podvm-builder-$(PODVM_DISTRO):$(PODVM_TAG)
PODVM_BINARIES_IMAGE ?= $(REGISTRY)/podvm-binaries-$(PODVM_DISTRO)-$(ARCH):$(PODVM_TAG)
PODVM_IMAGE ?= $(REGISTRY)/podvm-$(or $(CLOUD_PROVIDER),generic)-$(PODVM_DISTRO)-$(ARCH):$(PODVM_TAG)

PUSH ?= false
# If not pushing `--load` into the local docker cache
DOCKER_OPTS := $(if $(filter $(PUSH),true),--push,--load) $(EXTRA_DOCKER_OPTS)

DOCKERFILE_SUFFIX := $(if $(filter $(PODVM_DISTRO),ubuntu),,.$(PODVM_DISTRO))
BUILDER_DOCKERFILE := Dockerfile.podvm_builder$(DOCKERFILE_SUFFIX)
BINARIES_DOCKERFILE := Dockerfile.podvm_binaries$(DOCKERFILE_SUFFIX)
PODVM_DOCKERFILE := Dockerfile.podvm$(DOCKERFILE_SUFFIX)

podvm-builder:
	docker buildx build -t $(PODVM_BUILDER_IMAGE) -f podvm/$(BUILDER_DOCKERFILE) \
	--build-arg GO_VERSION=$(GO_VERSION) \
	--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
	--build-arg RUST_VERSION=$(RUST_VERSION) \
	--build-arg YQ_VERSION=$(YQ_VERSION) \
	--build-arg YQ_CHECKSUM=$(YQ_CHECKSUM) \
	$(DOCKER_OPTS) .

podvm-binaries:
	docker buildx build -t $(PODVM_BINARIES_IMAGE) -f podvm/$(BINARIES_DOCKERFILE) \
	--build-arg BUILDER_IMG=$(PODVM_BUILDER_IMAGE) \
	--build-arg PODVM_DISTRO=$(PODVM_DISTRO) \
	--build-arg ARCH=$(ARCH) \
	--build-arg AA_KBC=$(AA_KBC) \
	$(if $(DEFAULT_AGENT_POLICY_FILE),--build-arg DEFAULT_AGENT_POLICY_FILE=$(DEFAULT_AGENT_POLICY_FILE),) \
	$(DOCKER_OPTS) .

podvm-image:
	docker buildx build -t $(PODVM_IMAGE) -f podvm/$(PODVM_DOCKERFILE) \
	--build-arg BUILDER_IMG=$(PODVM_BUILDER_IMAGE) \
	--build-arg BINARIES_IMG=$(PODVM_BINARIES_IMAGE) \
	--build-arg PODVM_DISTRO=$(PODVM_DISTRO) \
	--build-arg ARCH=$(ARCH) \
	--build-arg CLOUD_PROVIDER=$(or $(CLOUD_PROVIDER),generic) \
	$(DOCKER_OPTS) .
