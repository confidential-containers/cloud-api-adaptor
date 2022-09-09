#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

.PHONY: build push verify ubuntu clean

UBUNTU_RELEASE     = focal
CONTAINERD_VERSION = 1.5.7
SKOPEO_VERSION     = 1.5.0
UMOCI_VERSION      = 0.4.7

IMAGE_PREFIX := podvm
ARCH := $(subst x86_64,amd64,$(shell uname -m))

FILES_DIR := files
FILES      = $(shell find "$(FILES_DIR)" -type f -o -type l)

ifndef IMAGE_NAME
COMMIT := $(shell	commit=$$(git describe --match '' --dirty --always) && \
					if [ -n "$$(git ls-files --other --exclude-per-directory .gitignore "$(FILES_DIR)")" ]; then \
						commit="$${commit%-dirty}-dirty"; \
					fi && \
	                echo "$$commit")
ifndef COMMIT
$(error Failed to derive an image name. Explicitly define IMAGE_NAME)
endif
IMAGE_NAME := $(IMAGE_PREFIX)-$(COMMIT)-$(ARCH)
endif
IMAGE_FILE := $(IMAGE_NAME).qcow2

UBUNTU_IMAGE_URL  := https://cloud-images.ubuntu.com/$(UBUNTU_RELEASE)/current/$(UBUNTU_RELEASE)-server-cloudimg-$(ARCH).img
UBUNTU_IMAGE_FILE := $(notdir $(UBUNTU_IMAGE_URL))

UBUNTU_PACKAGES = jq

AGENT_PROTOCOL_FORWARDER = $(FILES_DIR)/usr/local/bin/agent-protocol-forwarder
KATA_AGENT    = $(FILES_DIR)/usr/local/bin/kata-agent
SKOPEO        = $(FILES_DIR)/usr/bin/skopeo
UMOCI         = $(FILES_DIR)/usr/local/bin/umoci
PAUSE         = $(FILES_DIR)/$(PAUSE_BUNDLE)/rootfs/pause
BINARIES = $(AGENT_PROTOCOL_FORWARDER) $(KATA_AGENT) $(SKOPEO) $(UMOCI) $(PAUSE)

AGENT_PROTOCOL_FORWARDER_SRC = ../..

KATA_AGENT_SRC = ../../../kata-containers/src/agent
KATA_AGENT_BUILD_TYPE = release

SKOPEO_SRC  = skopeo
SKOPEO_REPO = https://github.com/containers/skopeo

UMOCI_SRC   = umoci
UMOCI_REPO  = https://github.com/opencontainers/umoci

# Embed the pause container image
# https://github.com/arronwy/kata-containers/commit/75b9f3fa3caaae62f49b4733f65cbab0cc87dbee
PAUSE_SRC     = pause
PAUSE_REPO    = docker://k8s.gcr.io/pause
PAUSE_VERSION = 3.6
PAUSE_BUNDLE  = pause_bundle

# Static libseccomp is necessary for kata-agent
# https://github.com/kata-containers/kata-containers/issues/5044#issuecomment-1239773921
STATIC_LIB_BUILDER = ../../../kata-containers/ci/install_libseccomp.sh
STATIC_LIB_DIR     = $(abspath staticlib)
STATIC_LIB         = $(STATIC_LIB_DIR)/kata-libseccomp/lib/libseccomp.a

build: $(IMAGE_FILE)

push: $(IMAGE_FILE)
	./push.sh --name "$(IMAGE_NAME)" --path "$(IMAGE_FILE)"

verify: push
	./verify.sh --image "$(IMAGE_NAME)"

SUDO =
WORDIR = .

$(IMAGE_FILE): $(UBUNTU_IMAGE_FILE) $(BINARIES) $(FILES)
	rm -f "$(IMAGE_FILE)"
	$(SUDO) ./build.sh  --root "$(FILES_DIR)" --packages "$(UBUNTU_PACKAGES)" --base "$(UBUNTU_IMAGE_FILE)" --output "$(IMAGE_FILE)"  --workdir "$(WORKDIR)"

$(AGENT_PROTOCOL_FORWARDER): force
	cd "$(AGENT_PROTOCOL_FORWARDER_SRC)" && $(MAKE) agent-protocol-forwarder
	install -D --compare "$(AGENT_PROTOCOL_FORWARDER_SRC)/agent-protocol-forwarder" "$@"

$(KATA_AGENT): force $(STATIC_LIB)
	cd "$(KATA_AGENT_SRC)" && $(MAKE) BUILD_TYPE=$(KATA_AGENT_BUILD_TYPE) LIBSECCOMP_LINK_TYPE=static LIBSECCOMP_LIB_PATH=$(dir $(STATIC_LIB))
	install -D --compare "$(KATA_AGENT_SRC)/target/$(shell uname -m)-unknown-linux-$(if $(findstring s390x,$(shell uname -m)),gnu,musl)/$(KATA_AGENT_BUILD_TYPE)/$(@F)" "$@"

$(STATIC_LIB):
	$(STATIC_LIB_BUILDER) $(STATIC_LIB_DIR)/kata-libseccomp $(STATIC_LIB_DIR)/kata-gperf

$(SKOPEO_SRC):
	git clone -b "v$(SKOPEO_VERSION)" "$(SKOPEO_REPO)" "$(SKOPEO_SRC)"

$(SKOPEO): $(SKOPEO_SRC)
	cd "$(SKOPEO_SRC)" && make bin/skopeo
	install -D --compare "$(SKOPEO_SRC)/bin/skopeo" "$@"

$(UMOCI_SRC):
	git clone -b "v$(UMOCI_VERSION)" "$(UMOCI_REPO)" "$(UMOCI_SRC)"

$(UMOCI): $(UMOCI_SRC)
	cd "$(UMOCI_SRC)" && make
	install -D --compare "$(UMOCI_SRC)/umoci" "$@"

$(PAUSE_SRC):
	$(SKOPEO) --policy "$(FILES_DIR)/etc/containers/policy.json" copy "$(PAUSE_REPO):$(PAUSE_VERSION)" "oci:$(PAUSE_SRC):$(PAUSE_VERSION)"

$(PAUSE): | $(PAUSE_SRC)
	$(UMOCI) unpack --image "$(PAUSE_SRC):$(PAUSE_VERSION)" "${FILES_DIR}/$(PAUSE_BUNDLE)"

ubuntu: $(UBUNTU_IMAGE_FILE)
$(UBUNTU_IMAGE_FILE):
	curl -s -L -o "$@" "$(UBUNTU_IMAGE_URL)"

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
