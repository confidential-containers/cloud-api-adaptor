# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

.PHONY: clean provisioner-cli

ARCH        ?= $(subst x86_64,amd64,$(shell uname -m))
GOOPTIONS   ?= GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0
GOFLAGS     ?= -tags=$(aws azure ibmcloud vsphere libvirt)
BINARIES    := caa-provisioner-cli

.PHONY: provisioner-cli
provisioner-cli: ## Build provisioner-cli for developer
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=1 go build $(GOFLAGS) -o caa-provisioner-cli "provisioner-cli/main.go"

.PHONY: clean
clean: ## Remove binaries.
	rm -f $(BINARIES)
