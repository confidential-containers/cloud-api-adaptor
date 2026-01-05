# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

.PHONY: all clean force

ARCH                    ?= $(subst x86_64,amd64,$(shell uname -m))
BUILTIN_CLOUD_PROVIDERS ?= aws azure ibmcloud libvirt
GOFLAGS                 ?=
GOOS                    ?= $(shell uname | tr A-Z a-z)
GOOPTIONS               ?= GOOS=$(GOOS) GOARCH=$(ARCH) CGO_ENABLED=0
BINARIES                := caa-provisioner-cli

space := $() $()
comma := ,
GOFLAGS += -tags=$(subst $(space),$(comma),$(strip $(BUILTIN_CLOUD_PROVIDERS)))

ifneq (,$(filter libvirt,$(BUILTIN_CLOUD_PROVIDERS)))
GOOPTIONS := $(subst CGO_ENABLED=0,CGO_ENABLED=1,$(GOOPTIONS))
endif

all: $(BINARIES)

.PHONY: provisioner-cli
provisioner-cli: caa-provisioner-cli

caa-provisioner-cli: force ## Build provisioner-cli for developer
	$(GOOPTIONS) go build $(GOFLAGS) -o caa-provisioner-cli ./provisioner-cli/

.PHONY: clean
clean: ## Remove binaries.
	rm -f $(BINARIES)
