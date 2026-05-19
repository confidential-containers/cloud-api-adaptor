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
BINARIES    := cloud-api-adaptor agent-protocol-forwarder process-user-data az-copy-image azure-ready
SOURCEDIRS  := ./cmd ./pkg
PACKAGES    := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES     := $(shell find $(SOURCEDIRS) -name '*.go' -print)
# End-to-end tests overall run timeout.
TEST_E2E_TIMEOUT ?= 60m
# To run a sub-set of tests, set an unanchored regex matching the test's name as in go's test -run option.
RUN_TESTS ?= ''

RESOURCE_CTRL ?= true
WEBHOOK_ENABLED ?= true
# BUILTIN_CLOUD_PROVIDERS is used for binary build -- what providers are built in the binaries.
ifeq ($(RELEASE_BUILD),true)
	BUILTIN_CLOUD_PROVIDERS ?= alibabacloud aws azure byom gcp ibmcloud ibmcloud_powervs
else
	BUILTIN_CLOUD_PROVIDERS ?= alibabacloud aws azure byom gcp ibmcloud ibmcloud_powervs libvirt
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

.PHONY: versions-check
versions-check: ## Validate versions.yaml against the JSON schema
	./hack/versions.py -c versions.schema.json

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests for single provider.
ifneq ($(CLOUD_PROVIDER),)
	go test -v -tags=$(CLOUD_PROVIDER) -timeout $(TEST_E2E_TIMEOUT) -count=1 -run $(RUN_TESTS) ./test/e2e
else
	$(error CLOUD_PROVIDER is not set)
endif

.PHONY: provision-cluster
provision-cluster: ## Provision the cluster without running tests or installing CAA.
ifndef TEST_PROVISION_FILE
	$(warning TEST_PROVISION_FILE is not set - some providers may require a properties file)
endif
	TEST_PROVISION=yes TEST_INSTALL_CAA=no TEST_TEARDOWN=no $(MAKE) test-e2e RUN_TESTS='^$$'

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

##@ Deployment

.PHONY: deploy
deploy: ## Deploy cloud-api-adaptor using Helm charts. Assumes provider secrets file exists at install/charts/peerpods/providers/$(CLOUD_PROVIDER)-secrets.yaml
ifneq ($(CLOUD_PROVIDER),)
	$(eval HELM_VALUES_FILES := install/charts/peerpods/providers/$(CLOUD_PROVIDER).yaml)
	$(eval HELM_VALUES_FILES := $(HELM_VALUES_FILES) install/charts/peerpods/providers/$(CLOUD_PROVIDER)-secrets.yaml)

ifeq ($(WEBHOOK_ENABLED),true)
	@if ! kubectl get svc -n cert-manager cert-manager-webhook > /dev/null 2>&1; then \
		echo "Installing cert-manager" ;\
		$(MAKE) -C ../webhook deploy-cert-manager ;\
	fi
endif
	helm install peerpods install/charts/peerpods \
		$(foreach file,$(HELM_VALUES_FILES),-f $(file)) \
		--set webhook.enabled=$(WEBHOOK_ENABLED) \
		--set resourceCtrl.enabled=$(RESOURCE_CTRL) \
		--dependency-update \
		-n confidential-containers-system \
		--create-namespace
else
	$(error CLOUD_PROVIDER is not set)
endif

.PHONY: delete
delete: ## Delete cloud-api-adaptor using Helm charts.
	@if helm status peerpods -n confidential-containers-system >/dev/null 2>&1; then \
		echo "Uninstalling peerpods release..."; \
		helm uninstall peerpods -n confidential-containers-system; \
	else \
		echo "Release 'peerpods' not found in namespace 'confidential-containers-system'(either not installed or already uninstalled)"; \
	fi

##@ Helm

.PHONY: static-helm-check
static-helm-check: ## Run static checks on helm chart (lint and template rendering)
	@echo "Building helm chart dependencies..."
	@helm dependency build install/charts/peerpods/
	@echo "Linting and rendering helm chart with all provider configurations..."
	@for provider in install/charts/peerpods/providers/*.yaml; do \
		if [ -f "$$provider" ] && [[ ! "$$provider" =~ template$$ ]]; then \
			echo "  $$(basename $$provider)"; \
			helm lint install/charts/peerpods/ -f "$$provider" || exit 1; \
			helm template test-release install/charts/peerpods/ -f "$$provider" > /dev/null || exit 1; \
		fi \
	done
	@echo "Static helm checks passed"
