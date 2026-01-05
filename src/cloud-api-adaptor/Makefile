# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

include Makefile.defaults

.PHONY: all build vet clean image deploy delete

SHELL = bash -o pipefail

# Default is dev build. To create release build set RELEASE_BUILD=true
RELEASE_BUILD ?= false
# CLOUD_PROVIDER is used for runtime -- which provider should be run against the binary/code.
CLOUD_PROVIDER ?=
GOOPTIONS   ?= GOOS=linux GOARCH=$(GO_ARCH) CGO_ENABLED=0
GOFLAGS     ?=
BINARIES    := cloud-api-adaptor agent-protocol-forwarder process-user-data az-copy-image
SOURCEDIRS  := ./cmd ./pkg
PACKAGES    := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES     := $(shell find $(SOURCEDIRS) -name '*.go' -print)
# End-to-end tests overall run timeout.
TEST_E2E_TIMEOUT ?= 60m
# To run a sub-set of tests, set an unanchored regex matching the test's name as in go's test -run option.
RUN_TESTS ?= ''

RESOURCE_CTRL ?= true
# BUILTIN_CLOUD_PROVIDERS is used for binary build -- what providers are built in the binaries.
ifeq ($(RELEASE_BUILD),true)
	BUILTIN_CLOUD_PROVIDERS ?= alibabacloud aws azure gcp ibmcloud ibmcloud_powervs
else
	BUILTIN_CLOUD_PROVIDERS ?= alibabacloud aws azure byom gcp ibmcloud ibmcloud_powervs libvirt docker
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

GOFLAGS += -ldflags="-X 'github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/cmd.VERSION=$(VERSION)' \
                     -X 'github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/cmd.COMMIT=$(COMMIT)'"

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
	go test -v -tags=$(CLOUD_PROVIDER) -timeout $(TEST_E2E_TIMEOUT) -count=1 -run $(RUN_TESTS) ./test/e2e
else
	$(error CLOUD_PROVIDER is not set)
endif

.PHONY: vet
vet: .git-commit ## Run go vet against code.
	go vet $(GOFLAGS) $(PACKAGES)

.PHONY: clean
clean: ## Remove binaries.
	rm -fr $(BINARIES) \
		.git-commit .git-commit.tmp

##@ Build

.PHONY: image
image: .git-commit ## Build and push docker image to $registry
	COMMIT=$(COMMIT) VERSION=$(VERSION) YQ_VERSION=$(YQ_VERSION) hack/build.sh -i

.PHONY: image-with-arch
image-with-arch: .git-commit ## Build the per arch image
	COMMIT=$(COMMIT) VERSION=$(VERSION) YQ_VERSION=$(YQ_VERSION) hack/build.sh -a

##@ Deployment

.PHONY: deploy
deploy: ## Deploy cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml file.
ifneq ($(CLOUD_PROVIDER),)
	kubectl apply -k "$(COCO_OPERATOR_REPO)/config/$(COCO_OPERATOR_CONFIG)?ref=$(COCO_OPERATOR_REF)"
	kubectl apply -k "$(COCO_OPERATOR_REPO)/config/samples/ccruntime/peer-pods?ref=$(COCO_OPERATOR_REF)"
	kubectl apply -k install/overlays/$(CLOUD_PROVIDER)
else
	$(error CLOUD_PROVIDER is not set)
endif
ifeq ($(RESOURCE_CTRL),true)
	$(MAKE) -C ../peerpod-ctrl deploy
	@if kubectl get svc -n cert-manager cert-manager-webhook > /dev/null 2>&1; then \
		echo "Installing webhook" ;\
		$(MAKE) -C ../webhook deploy ;\
	else \
		echo "Installing cert-manager" ;\
		$(MAKE) -C ../webhook deploy-cert-manager ;\
		echo "Installing webhook" ;\
		$(MAKE) -C ../webhook deploy ;\
	fi
endif

.PHONY: delete
delete: ## Delete cloud-api-adaptor using the operator, according to install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml file.
ifeq ($(RESOURCE_CTRL),true)
	$(MAKE) -C ../peerpod-ctrl undeploy
	$(MAKE) -C ../webhook undeploy
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
PODVM_BINARIES_IMAGE ?= $(REGISTRY)/podvm-binaries-$(PODVM_DISTRO)-$(DISTRO_ARCH):$(PODVM_TAG)
PODVM_BYOM_BINARIES_IMAGE ?= $(REGISTRY)/podvm-byom-binaries-$(PODVM_DISTRO)-$(DISTRO_ARCH):$(PODVM_TAG)
PODVM_IMAGE ?= $(REGISTRY)/podvm-$(or $(CLOUD_PROVIDER),generic)-$(PODVM_DISTRO)-$(DISTRO_ARCH):$(PODVM_TAG)

PUSH ?= false
# If not pushing `--load` into the local docker cache
DOCKER_OPTS := $(if $(filter $(PUSH),true),--push,--load) $(EXTRA_DOCKER_OPTS)

DOCKERFILE_SUFFIX := $(if $(filter $(PODVM_DISTRO),ubuntu alinux),,.$(PODVM_DISTRO))
BUILDER_DOCKERFILE := Dockerfile.podvm_builder$(DOCKERFILE_SUFFIX)
BINARIES_DOCKERFILE := Dockerfile.podvm_binaries$(DOCKERFILE_SUFFIX)
BYOM_BINARIES_DOCKERFILE := Dockerfile.podvm_byom_binaries$(DOCKERFILE_SUFFIX)
PODVM_DOCKERFILE := Dockerfile.podvm$(DOCKERFILE_SUFFIX)

podvm-builder:
	docker buildx build -t $(PODVM_BUILDER_IMAGE) -f podvm/$(BUILDER_DOCKERFILE) \
	--build-arg GO_VERSION=$(GO_VERSION) \
	--build-arg ORG_ID=$(ORG_ID) \
	--build-arg ACTIVATION_KEY=$(ACTIVATION_KEY) \
	--build-arg ARCH=$(ARCH) \
	--build-arg GO_ARCH=$(GO_ARCH) \
	--build-arg DISTRO_ARCH=$(DISTRO_ARCH) \
	--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
	--build-arg YQ_VERSION=$(YQ_VERSION) \
	--build-arg YQ_CHECKSUM=${YQ_CHECKSUM_$(GO_ARCH)} \
	--build-arg YQ_ARCH=$(GO_ARCH) \
	--build-arg PROTOC_ARCH=$(PROTOC_ARCH) \
	--build-arg ORAS_VERSION=$(ORAS_VERSION) \
	--build-arg PACKER_VERSION=$(PACKER_VERSION) \
	$(DOCKER_OPTS) .

podvm-binaries:
	cp -rf ../../.git .git
	cd ../ && docker buildx build -t $(PODVM_BINARIES_IMAGE) -f cloud-api-adaptor/podvm/$(BINARIES_DOCKERFILE) \
	--build-arg BUILDER_IMG=$(PODVM_BUILDER_IMAGE) \
	--build-arg PODVM_DISTRO=$(PODVM_DISTRO) \
	--build-arg ARCH=$(ARCH) \
	--build-arg GO_ARCH=$(GO_ARCH) \
	--build-arg DISTRO_ARCH=$(DISTRO_ARCH) \
	--build-arg TEE_PLATFORM=$(TEE_PLATFORM) \
	--build-arg PAUSE_REPO=$(PAUSE_REPO) \
	--build-arg PAUSE_VERSION=$(PAUSE_VERSION) \
	--build-arg PAUSE_BIN=$(PAUSE_BIN) \
	$(if $(AUTHFILE),--build-arg AUTHFILE=$(AUTHFILE),) \
	$(if $(DEFAULT_AGENT_POLICY_FILE),--build-arg DEFAULT_AGENT_POLICY_FILE=$(DEFAULT_AGENT_POLICY_FILE),) \
	$(DOCKER_OPTS) .
	rm -rf .git

podvm-image:
ifeq ($(PODVM_DISTRO),rhel)
ifeq ($(shell test -f $(IMAGE_URL) && echo 1 || echo 0),1)
	cp ${IMAGE_URL} ./rhel.img
	$(eval IMAGE_URL=cloud-api-adaptor/rhel.img)
	@echo "### Copied IMAGE_URL file to $(IMAGE_URL) in order to suite the docker context"
endif
endif
	cp -rf ../../.git .git
	cd ../ && docker buildx build -t $(PODVM_IMAGE) -f cloud-api-adaptor/podvm/$(PODVM_DOCKERFILE) \
	--build-arg BUILDER_IMG=$(PODVM_BUILDER_IMAGE) \
	--build-arg BINARIES_IMG=$(PODVM_BINARIES_IMAGE) \
	--build-arg PODVM_DISTRO=$(PODVM_DISTRO) \
	--build-arg ORG_ID=$(ORG_ID) \
	--build-arg ACTIVATION_KEY=$(ACTIVATION_KEY) \
	--build-arg ARCH=$(ARCH) \
	--build-arg CLOUD_PROVIDER=$(or $(CLOUD_PROVIDER),generic) \
	--build-arg IMAGE_URL=$(IMAGE_URL) \
	--build-arg IMAGE_CHECKSUM=$(IMAGE_CHECKSUM) \
	--build-arg SE_BOOT=$(SE_BOOT) \
	--build-arg SE_VERIFY=$(SE_VERIFY) \
	--build-arg PAUSE_REPO=$(PAUSE_REPO) \
	--build-arg PAUSE_VERSION=$(PAUSE_VERSION) \
	--build-arg PAUSE_BIN=$(PAUSE_BIN) \
	--build-arg PACKER_GITHUB_API_TOKEN=$(PACKER_GITHUB_API_TOKEN) \
	$(if $(AUTHFILE),--build-arg AUTHFILE=$(AUTHFILE),) \
	$(DOCKER_OPTS) .
	rm -rf .git

podvm-byom-binaries-image:
	docker buildx build -t $(PODVM_BYOM_BINARIES_IMAGE) -f podvm-mkosi/$(BYOM_BINARIES_DOCKERFILE) \
	--build-arg BINARIES_IMG=$(PODVM_BINARIES_IMAGE) \
	$(DOCKER_OPTS) .
