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

HOST_ARCH   := $(shell uname -m)
ARCH        := $(or $(ARCH),$(HOST_ARCH))
# Normalise x86_64 / amd64 for input ARCH
ARCH        := $(subst amd64,x86_64,$(ARCH))
DEB_ARCH    := $(subst x86_64,amd64,$(ARCH))
LIBC        ?= $(if $(filter $(ARCH),s390x ppc64le),gnu,musl)
RUST_ARCH   ?= $(subst ppc64le,powerpc64le,$(ARCH))
RUST_TARGET := $(RUST_ARCH)-unknown-linux-$(LIBC)

# Auth json file for registry access. Used with skopeo
AUTHFILE ?=

ATTESTER ?= none
CDH_RESOURCE_PROVIDER ?= kbs
SEALED_SECRET ?= yes

AGENT_POLICY ?= yes
AGENT_POLICY_PATH := $(FILES_DIR)/etc/kata-opa
DEFAULT_AGENT_POLICY_FILE ?= allow-all.rego


FORCE_TARGET := $(if $(FORCE),force,)

SHELL := env PATH=$(PATH) /bin/bash

ifneq ($(HOST_ARCH),$(ARCH))
    ifeq ($(CC),cc)
		CC := $(ARCH)-linux-$(LIBC)-gcc
        $(warning A foreign ARCH was passed, but no CC alternative. Using $(CC) as best guess)
	endif
RUST_FLAGS := CC=$(CC) ARCH=$(ARCH)
endif

ifndef IMAGE_NAME
COMMIT := $(shell	commit=$$(git describe --match '' --dirty --always) && \
					if [ -n "$$(git ls-files --other --exclude-per-directory .gitignore "$(FILES_DIR)")" ]; then \
						commit="$${commit%-dirty}-dirty"; \
					fi && \
	                echo "$$commit")
ifndef COMMIT
$(error Failed to derive an image name. Explicitly define IMAGE_NAME)
endif
IMAGE_NAME := $(IMAGE_PREFIX)-$(COMMIT)-$(DEB_ARCH)
endif

IMAGE_SUFFIX ?=
IMAGE_FILE = $(IMAGE_NAME)$(IMAGE_SUFFIX)

AGENT_PROTOCOL_FORWARDER = $(FILES_DIR)/usr/local/bin/agent-protocol-forwarder
KATA_AGENT    = $(FILES_DIR)/usr/local/bin/kata-agent

# The pause binary can be named differently.
PAUSE_BIN ?= pause
PAUSE      = $(FILES_DIR)/$(PAUSE_BUNDLE)/rootfs/$(PAUSE_BIN)
# Allows to override
PAUSE_BUNDLE  ?= pause_bundle

ATTESTATION_AGENT = $(FILES_DIR)/usr/local/bin/attestation-agent
CONFIDENTIAL_DATA_HUB = $(FILES_DIR)/usr/local/bin/confidential-data-hub
API_SERVER_REST  = $(FILES_DIR)/usr/local/bin/api-server-rest
PROCESS_USER_DATA = $(FILES_DIR)/usr/local/bin/process-user-data


# Allow BINARIES to be overriden externally

BINARIES ?= $(AGENT_PROTOCOL_FORWARDER) $(KATA_AGENT) $(PAUSE) $(ATTESTATION_AGENT) $(CONFIDENTIAL_DATA_HUB) $(API_SERVER_REST) $(PROCESS_USER_DATA)

GUEST_COMPONENTS = ../guest-components

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
define generate_tag
	$(eval $0_tag_length_limit=$(shell expr 128 - $(shell echo "-$(ARCH)" | wc -c)))
	processed_tag=$(shell echo $(2) | tr -dc '[:print:]' | tr -c '[a-zA-Z0-9\_\.\-]' _ | head -c $(shell echo $($0_tag_length_limit)))
	$(1) := $$(processed_tag)-$(ARCH)
endef

binaries: $(BINARIES)

$(AGENT_PROTOCOL_FORWARDER): always
	cd "$(ROOT_DIR)" && ARCH=$(DEB_ARCH) $(MAKE) agent-protocol-forwarder
	install -D --compare "$(ROOT_DIR)/agent-protocol-forwarder" "$@"

$(PROCESS_USER_DATA): always
	cd "$(ROOT_DIR)" && ARCH=$(DEB_ARCH) $(MAKE) process-user-data
	install -D --compare "$(ROOT_DIR)/process-user-data" "$@"

$(KATA_AGENT): $(FORCE_TARGET)
	$(eval $(call generate_tag,tag,$(KATA_SRC_REF)))
	oras pull ghcr.io/kata-containers/cached-artefacts/agent:${tag}
	tar xvJpf kata-static-agent.tar.xz
	install -D --compare "./usr/bin/kata-agent" "$@"

$(PAUSE):
	$(eval $(call generate_tag,tag,$(PAUSE_REPO)-$(PAUSE_VERSION)))
	oras pull ghcr.io/kata-containers/cached-artefacts/pause-image:${tag}
	tar xvJpf kata-static-pause-image.tar.xz
	mkdir -p ${FILES_DIR}/$(PAUSE_BUNDLE)
	cp -R ./pause_bundle/* ${FILES_DIR}/$(PAUSE_BUNDLE)

$(GUEST_COMPONENTS):
	mkdir -p "$(GUEST_COMPONENTS)"
	$(eval $(call generate_tag,tag,$(GUEST_COMPONENTS_VERSION)))
	oras pull ghcr.io/kata-containers/cached-artefacts/coco-guest-components:${tag}
	tar xvJpf kata-static-coco-guest-components.tar.xz -C "$(GUEST_COMPONENTS)"

$(ATTESTATION_AGENT): $(FORCE_TARGET) | $(GUEST_COMPONENTS)
	install -D --compare "$(GUEST_COMPONENTS)/usr/local/bin/attestation-agent" "$@"

$(CONFIDENTIAL_DATA_HUB): $(FORCE_TARGET) | $(GUEST_COMPONENTS)
	install -D --compare "$(GUEST_COMPONENTS)/usr/local/bin/confidential-data-hub" "$@"

$(API_SERVER_REST): $(FORCE_TARGET) | $(GUEST_COMPONENTS)
	install -D --compare "$(GUEST_COMPONENTS)/usr/local/bin/api-server-rest" "$@"

clean_sources:
	[ -d "$(ROOT_DIR)" ] && cd "$(ROOT_DIR)" && git clean -xfd -e podvm

.PHONY: force always
always force:
