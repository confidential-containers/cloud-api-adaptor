#
# SPDX-License-Identifier: Apache-2.0
#

# This file contains the common build steps across the providers
# Including building the required binaries from source
# It is to be included via `include ../../podvm/Makefile.inc` in
# each of the cloud provider podvm image Makefiles

UBUNTU_RELEASE     = focal
SKOPEO_VERSION     = 1.5.0
UMOCI_VERSION      = 0.4.7
GUEST_COMPONENTS_VERSION ?= "v0.7.0"

IMAGE_PREFIX := podvm

HOST_ARCH   := $(shell uname -m)
ARCH        ?= $(HOST_ARCH)
# Normalise x86_64 / amd64 for input ARCH
ARCH        := $(subst amd64,x86_64,$(ARCH))
DEB_ARCH    := $(subst x86_64,amd64,$(ARCH))
AA_KBC      ?= offline_fs_kbc
KBC_URI     ?= null
LIBC        ?= $(if $(filter $(ARCH),s390x ppc64le),gnu,musl)
RUST_ARCH   ?= $(subst ppc64le,powerpc64le,$(ARCH))
RUST_TARGET := $(RUST_ARCH)-unknown-linux-$(LIBC)

ifneq ($(HOST_ARCH),$(ARCH))
    ifeq ($(CC),cc)
		CC := $(ARCH)-linux-$(LIBC)-gcc
        $(warning A foreign ARCH was passed, but no CC alternative. Using $(CC) as best guess)
	endif
RUST_FLAGS := CC=$(CC) ARCH=$(ARCH)
endif

FILES_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))/files

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
PAUSE         = $(FILES_DIR)/$(PAUSE_BUNDLE)/rootfs/pause
ATTESTATION_AGENT = $(FILES_DIR)/usr/local/bin/attestation-agent

# Allow BINARIES to be overriden externally
BINARIES ?= $(AGENT_PROTOCOL_FORWARDER) $(KATA_AGENT) $(PAUSE)

BINARIES += $(ATTESTATION_AGENT)
$(shell sed -i "s/\(aa_kbc_params = \)\"[^\"]*\"/\1\"${AA_KBC}::${KBC_URI}\"/g" $(FILES_DIR)/etc/agent-config.toml)

AGENT_PROTOCOL_FORWARDER_SRC = ../..

KATA_AGENT_SRC = ../../../kata-containers/src/agent
KATA_AGENT_BUILD_TYPE = release

SKOPEO_SRC ?= skopeo
SKOPEO_REPO = https://github.com/containers/skopeo
SKOPEO_BIN ?= $(SKOPEO_SRC)/bin/skopeo

UMOCI_SRC   = umoci
UMOCI_REPO  = https://github.com/opencontainers/umoci

# Embed the pause container image
# https://github.com/arronwy/kata-containers/commit/75b9f3fa3caaae62f49b4733f65cbab0cc87dbee
PAUSE_SRC     = pause
PAUSE_REPO    = docker://registry.k8s.io/pause
PAUSE_VERSION = 3.6
# Allows to override
PAUSE_BUNDLE  ?= pause_bundle

# Static libseccomp is necessary for kata-agent
# https://github.com/kata-containers/kata-containers/issues/5044#issuecomment-1239773921
STATIC_LIBSECCOMP_BUILDER = ../../../kata-containers/ci/install_libseccomp.sh
STATIC_LIBSECCOMP_DIR     = $(abspath staticlib)
STATIC_LIBSECCOMP         = $(STATIC_LIBSECCOMP_DIR)/kata-libseccomp/lib/libseccomp.a

GUEST_COMPONENTS_SRC = ../../../guest-components
GUEST_COMPONENTS_REPO ?= https://github.com/confidential-containers/guest-components

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

binaries: $(BINARIES)

$(AGENT_PROTOCOL_FORWARDER): force
	cd "$(AGENT_PROTOCOL_FORWARDER_SRC)" && ARCH=$(DEB_ARCH) $(MAKE) agent-protocol-forwarder
	install -D --compare "$(AGENT_PROTOCOL_FORWARDER_SRC)/agent-protocol-forwarder" "$@"

$(KATA_AGENT): force $(STATIC_LIBSECCOMP)
	cd "$(KATA_AGENT_SRC)" && $(RUST_FLAGS) $(MAKE) BUILD_TYPE=$(KATA_AGENT_BUILD_TYPE) LIBSECCOMP_LINK_TYPE=static LIBSECCOMP_LIB_PATH=$(dir $(STATIC_LIBSECCOMP))
	install -D --compare "$(KATA_AGENT_SRC)/target/$(RUST_TARGET)/$(KATA_AGENT_BUILD_TYPE)/$(@F)" "$@"

$(STATIC_LIBSECCOMP):
	$(RUST_FLAGS) $(STATIC_LIBSECCOMP_BUILDER) $(STATIC_LIBSECCOMP_DIR)/kata-libseccomp $(STATIC_LIBSECCOMP_DIR)/kata-gperf

# Skopeo package packages are available in RHEL/CentOS 8 or later and Ubuntu 20.10 or later
$(SKOPEO_SRC):
	git clone -b "v$(SKOPEO_VERSION)" "$(SKOPEO_REPO)" "$(SKOPEO_SRC)"

$(SKOPEO_BIN): $(SKOPEO_SRC)
	cd "$(SKOPEO_SRC)" && CC= $(MAKE) bin/skopeo

# The umoci release page only publishes amd64 binaries. https://github.com/opencontainers/umoci/releases
$(UMOCI_SRC):
	git clone -b "v$(UMOCI_VERSION)" "$(UMOCI_REPO)" "$(UMOCI_SRC)"

$(UMOCI_SRC)/umoci: $(UMOCI_SRC)
	cd "$(UMOCI_SRC)" && CC= $(MAKE)

$(PAUSE_SRC): $(SKOPEO_BIN)
	$(SKOPEO_BIN) --override-arch $(DEB_ARCH) --policy "$(FILES_DIR)/etc/containers/policy.json" copy "$(PAUSE_REPO):$(PAUSE_VERSION)" "oci:$(PAUSE_SRC):$(PAUSE_VERSION)"

$(PAUSE): | $(PAUSE_SRC) $(UMOCI_SRC)/umoci
	$(UMOCI_SRC)/umoci unpack --rootless --image "$(PAUSE_SRC):$(PAUSE_VERSION)" "${FILES_DIR}/$(PAUSE_BUNDLE)"

$(GUEST_COMPONENTS_SRC):
	$(call git_clone_repo_ref,$(GUEST_COMPONENTS_REPO),$(GUEST_COMPONENTS_SRC),$(GUEST_COMPONENTS_VERSION))

$(ATTESTATION_AGENT): $(GUEST_COMPONENTS_SRC)
	cd "$(GUEST_COMPONENTS_SRC)/attestation-agent" && CC= ARCH=$(ARCH) $(MAKE) KBC="$(AA_KBC)" ttrpc=true LIBC="$(LIBC)"
	mkdir -p "$(@D)"
	install --compare "$(GUEST_COMPONENTS_SRC)/target/$(RUST_TARGET)/release/attestation-agent" "$@"

clean_sources:
	[ -d "$(GUEST_COMPONENTS_SRC)" ] && cd "$(GUEST_COMPONENTS_SRC)" && git clean -xfd
	[ -d "$(KATA_AGENT_SRC)" ] && cd "$(KATA_AGENT_SRC)" && git clean -xfd
	[ -d "$(AGENT_PROTOCOL_FORWARDER_SRC)" ] && cd "$(AGENT_PROTOCOL_FORWARDER_SRC)" && git clean -xfd -e podvm
	[ -d "$(PAUSE_SRC)" ] && cd "$(PAUSE_SRC)" && rm -rf *
	[ -d "$(UMOCI_SRC)" ] && cd "$(UMOCI_SRC)" && git clean -xfd -e umoci
