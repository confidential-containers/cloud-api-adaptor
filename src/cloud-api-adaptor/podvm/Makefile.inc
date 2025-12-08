#
# SPDX-License-Identifier: Apache-2.0
#

# This file contains the common build steps across the providers
# Including building the required binaries from source
# It can be included via
# ```
# ROOT_DIR := $(dir $(lastword $(MAKEFILE_LIST)))../../
# include $(ROOT_DIR)podvm/Makefile.inc
# ```
# in each of the cloud provider podvm image Makefiles

ROOT_DIR := $(dir $(lastword $(MAKEFILE_LIST)))../
FILES_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))/files

include $(ROOT_DIR)Makefile.defaults

IMAGE_PREFIX := podvm

# Auth json file for registry access. Used with skopeo
AUTHFILE ?=

TEE_PLATFORM ?= none
CDH_RESOURCE_PROVIDER ?= kbs
SEALED_SECRET ?= yes

AGENT_POLICY ?= yes
AGENT_POLICY_PATH := $(FILES_DIR)/etc/kata-opa
DEFAULT_AGENT_POLICY_FILE ?= allow-all.rego

# Run github artifact provenance verifications
VERIFY_PROVENANCE ?= no
GUEST_COMPONENTS_REPO := confidential-containers/guest-components
KATA_REPO := kata-containers/kata-containers

ifdef GITHUB_TOKEN
GH_AUTH_HEADER := -H "Authorization: Bearer $(GITHUB_TOKEN)"
endif
GH_API_BASE := https://api.github.com
GH_CURL := curl --fail --silent --show-error --retry 3 $(GH_AUTH_HEADER)

# probe version if the ref does not appear to be a git commit digest
ifeq ($(shell echo $(GUEST_COMPONENTS_REF) | grep -E "^[0-9a-f]{40}$$"),)
GUEST_COMPONENTS_COMMIT := $(shell \
	$(GH_CURL) \
	$(GH_API_BASE)/repos/$(GUEST_COMPONENTS_REPO)/commits/v$(GUEST_COMPONENTS_REF) \
	| jq -r '.sha // empty')
ifeq ($(GUEST_COMPONENTS_COMMIT),)
$(error jq failed to extract SHA from ref $(GUEST_COMPONENTS_REF))
endif
else
GUEST_COMPONENTS_COMMIT := $(GUEST_COMPONENTS_REF)
endif

# probe version if the ref does not appear to be a git commit digest
ifeq ($(shell echo $(KATA_REF) | grep -E "^[0-9a-f]{40}$$"),)
KATA_COMMIT := $(shell curl \
	$(GH_CURL) \
	$(GH_API_BASE)/repos/$(KATA_REPO)/commits/$(KATA_REF) \
	| jq -r '.sha // empty')
ifeq ($(KATA_COMMIT),)
$(error jq failed to extract SHA from ref $(KATA_REF))
endif
else
KATA_COMMIT := $(KATA_REF)
endif

FORCE_TARGET := $(if $(FORCE),force,)

SHELL := env PATH=$(PATH) /bin/bash

ifndef IMAGE_NAME
COMMIT := $(shell	commit=$$(git describe --match '' --dirty --always) && \
					if [ -n "$$(git ls-files --other --exclude-per-directory .gitignore "$(FILES_DIR)")" ]; then \
						commit="$${commit%-dirty}-dirty"; \
					fi && \
	                echo "$$commit")
ifndef COMMIT
$(error Failed to derive an image name. Explicitly define IMAGE_NAME)
endif
IMAGE_NAME := $(IMAGE_PREFIX)-$(COMMIT)-$(DISTRO_ARCH)
endif

IMAGE_SUFFIX ?=
IMAGE_FILE = $(IMAGE_NAME)$(IMAGE_SUFFIX)

AGENT_PROTOCOL_FORWARDER = $(FILES_DIR)/usr/local/bin/agent-protocol-forwarder
KATA_AGENT    = $(FILES_DIR)/usr/local/bin/kata-agent

# The pause binary can be named differently.
PAUSE_BIN ?= pause
PAUSE      = $(FILES_DIR)/$(PAUSE_BUNDLE)/rootfs/$(PAUSE_BIN)

ATTESTATION_AGENT = $(FILES_DIR)/usr/local/bin/attestation-agent
CONFIDENTIAL_DATA_HUB = $(FILES_DIR)/usr/local/bin/confidential-data-hub
API_SERVER_REST  = $(FILES_DIR)/usr/local/bin/api-server-rest
PROCESS_USER_DATA = $(FILES_DIR)/usr/local/bin/process-user-data


# Allow BINARIES to be overriden externally

BINARIES ?= \
	$(AGENT_PROTOCOL_FORWARDER) \
	$(API_SERVER_REST) \
	$(ATTESTATION_AGENT) \
	$(CONFIDENTIAL_DATA_HUB) \
	$(KATA_AGENT) \
	$(PAUSE) \
	$(PROCESS_USER_DATA)

SKOPEO_SRC ?= skopeo
SKOPEO_BIN ?= $(SKOPEO_SRC)/bin/skopeo

UMOCI_SRC   = umoci

# Embed the pause container image
# https://github.com/arronwy/kata-containers/commit/75b9f3fa3caaae62f49b4733f65cbab0cc87dbee
PAUSE_SRC     = pause
# Allows to override
PAUSE_BUNDLE  ?= pause_bundle

define pull_agent_artifact
	$(eval $(call generate_tag,tag,$(KATA_REF),$(ARCH)))
	$(eval OCI_IMAGE := $(KATA_REGISTRY)/agent)
	$(eval OCI_DIGEST := $(shell oras resolve $(OCI_IMAGE):${tag}))
	$(eval OCI_REF := $(OCI_IMAGE)@$(OCI_DIGEST))
	$(if $(filter yes,$(VERIFY_PROVENANCE)),$(ROOT_DIR)hack/verify-provenance.sh \
		-a $(OCI_REF) \
		-r $(KATA_REPO) \
		-d $(KATA_COMMIT))
	oras pull $(OCI_REF)
endef

# $(1) - Artifact name
# $(2) - Tag suffix
define pull_gc_artifact
	$(eval $(call generate_tag,tag,$(GUEST_COMPONENTS_REF),$(2)))
	$(eval OCI_IMAGE := $(GUEST_COMPONENTS_REGISTRY)/$(1))
	$(eval OCI_DIGEST := $(shell oras resolve $(OCI_IMAGE):${tag}))
	$(eval OCI_REF := $(OCI_IMAGE)@$(OCI_DIGEST))
	$(if $(filter yes,$(VERIFY_PROVENANCE)),$(ROOT_DIR)hack/verify-provenance.sh \
		-a $(OCI_REF) \
		-r $(GUEST_COMPONENTS_REPO) \
		-d $(GUEST_COMPONENTS_COMMIT))
	oras pull $(OCI_REF)
endef

# Clone a specific commit/tag/branch of a repo.
# $(1) - Repository URL
# $(2) - Destination directory
# $(3) - Commit/tag/branch
define git_clone_repo_ref
	@echo "Cloning $(1) to $(2)"
	mkdir -p "$(2)" && \
		cd "$(2)" && \
		git init && \
		git remote add origin "$(1)" && \
		git fetch --depth=1 origin "$(3)" && \
		git reset --hard FETCH_HEAD
endef

# Ensure that the oras tags is converted to something valid
# $(1) - return value
# $(2) - initial_tag
# $(3) - suffix
define generate_tag
	$(eval $0_tag_length_limit=$(shell expr 128 - $(shell echo "-$(3)" | wc -c)))
	processed_tag=$(shell echo $(2) | tr -dc '[:print:]' | tr -c '[a-zA-Z0-9\_\.\-]' _ | head -c $(shell echo $($0_tag_length_limit)))
	$(1) := $$(processed_tag)-$(3)
endef

binaries: $(BINARIES)

$(AGENT_PROTOCOL_FORWARDER): always
	cd "$(ROOT_DIR)" && ARCH=$(ARCH) $(MAKE) agent-protocol-forwarder
	install -D --compare "$(ROOT_DIR)/agent-protocol-forwarder" "$@"

$(PROCESS_USER_DATA): always
	cd "$(ROOT_DIR)" && ARCH=$(ARCH) $(MAKE) process-user-data
	install -D --compare "$(ROOT_DIR)/process-user-data" "$@"

$(KATA_AGENT): $(FORCE_TARGET)
	$(call pull_agent_artifact)
	tar --zstd -xvpf kata-static-agent.tar.zst
	install -D --compare "./usr/bin/kata-agent" "$@"

# Skopeo package packages are available in RHEL/CentOS 8 or later and Ubuntu 20.10 or later
$(SKOPEO_SRC):
	$(call git_clone_repo_ref,$(SKOPEO_REPO),$(SKOPEO_SRC),$(SKOPEO_VERSION))

$(SKOPEO_BIN): $(SKOPEO_SRC)
ifeq ($(ARCH),s390x)
	cd "$(SKOPEO_SRC)" && CC=gcc $(MAKE) bin/skopeo
else
	cd "$(SKOPEO_SRC)" && CC= $(MAKE) bin/skopeo
endif

# The umoci release page only publishes amd64 binaries. https://github.com/opencontainers/umoci/releases
$(UMOCI_SRC):
	$(call git_clone_repo_ref,$(UMOCI_REPO),$(UMOCI_SRC),$(UMOCI_VERSION))

$(UMOCI_SRC)/umoci: $(UMOCI_SRC)
	cd "$(UMOCI_SRC)" && CC= $(MAKE)

$(PAUSE_SRC): $(SKOPEO_BIN)
	$(SKOPEO_BIN) --override-arch $(DISTRO_ARCH) --policy "$(FILES_DIR)/etc/containers/policy.json" copy \
		$(if $(AUTHFILE),--authfile $(AUTHFILE)) "docker://$(PAUSE_REPO):$(PAUSE_VERSION)" "oci:$(PAUSE_SRC):$(PAUSE_VERSION)"

$(PAUSE): | $(PAUSE_SRC) $(UMOCI_SRC)/umoci
	$(UMOCI_SRC)/umoci unpack --rootless --image "$(PAUSE_SRC):$(PAUSE_VERSION)" "${FILES_DIR}/$(PAUSE_BUNDLE)"

$(ATTESTATION_AGENT): $(FORCE_TARGET)
	$(call pull_gc_artifact,attestation-agent,$(TEE_PLATFORM)_$(ARCH))
	tar xvJpf attestation-agent.tar.xz
	install -D --compare attestation-agent "$@"

$(CONFIDENTIAL_DATA_HUB): $(FORCE_TARGET)
	$(call pull_gc_artifact,confidential-data-hub,$(ARCH))
	tar xvJpf confidential-data-hub.tar.xz
	install -D --compare confidential-data-hub "$@"

$(API_SERVER_REST): $(FORCE_TARGET)
	$(call pull_gc_artifact,api-server-rest,$(ARCH))
	tar xvJpf api-server-rest.tar.xz
	install -D --compare api-server-rest "$@"

clean_sources:
	[ -d "$(ROOT_DIR)" ] && cd "$(ROOT_DIR)" && git clean -xfd -e podvm
	[ -d "$(PAUSE_SRC)" ] && cd "$(PAUSE_SRC)" && rm -rf *
	[ -d "$(UMOCI_SRC)" ] && cd "$(UMOCI_SRC)" && git clean -xfd -e umoci

.PHONY: force always
always force:
