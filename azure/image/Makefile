#
# SPDX-License-Identifier: Apache-2.0
#

.PHONY: build clean

UBUNTU_RELEASE     = focal
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
IMAGE_FILE := $(IMAGE_NAME)



AGENT_PROTOCOL_FORWARDER = $(FILES_DIR)/usr/local/bin/agent-protocol-forwarder
KATA_AGENT    = $(FILES_DIR)/usr/local/bin/kata-agent
PAUSE         = $(FILES_DIR)/$(PAUSE_BUNDLE)/rootfs/pause
SKOPEO    = $(FILES_DIR)/usr/bin/skopeo
UMOCI     = $(FILES_DIR)/usr/local/bin/umoci

BINARIES = $(AGENT_PROTOCOL_FORWARDER) $(KATA_AGENT) $(PAUSE)

ifdef USE_SKOPEO
BINARIES += $(SKOPEO) $(UMOCI)
endif

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

$(IMAGE_FILE): $(BINARIES) $(FILES)
	mkdir -p toupload
	packer build -var client_id=${CLIENT_ID} \
		-var client_secret=${CLIENT_SECRET} \
		-var subscription_id=${SUBSCRIPTION_ID} \
		-var tenant_id=${TENANT_ID} \
		-var location=${LOCATION} \
		-var vm_size=${VM_SIZE} \
		-var resource_group=${RESOURCE_GROUP} \
		-var az_image_name=${IMAGE_NAME} .
	rm -fr toupload

$(AGENT_PROTOCOL_FORWARDER): force
	cd "$(AGENT_PROTOCOL_FORWARDER_SRC)" && $(MAKE) agent-protocol-forwarder
	install -D --compare "$(AGENT_PROTOCOL_FORWARDER_SRC)/agent-protocol-forwarder" "$@"

$(KATA_AGENT): force $(STATIC_LIB)
	cd "$(KATA_AGENT_SRC)/../libs" && $(MAKE) BUILD_TYPE=$(KATA_AGENT_BUILD_TYPE) LIBSECCOMP_LINK_TYPE=static LIBSECCOMP_LIB_PATH=$(dir $(STATIC_LIB))
	cd "$(KATA_AGENT_SRC)" && $(MAKE) BUILD_TYPE=$(KATA_AGENT_BUILD_TYPE) LIBSECCOMP_LINK_TYPE=static LIBSECCOMP_LIB_PATH=$(dir $(STATIC_LIB))
	install -D --compare "$(KATA_AGENT_SRC)/target/$(shell uname -m)-unknown-linux-$(if $(findstring s390x,$(shell uname -m)),gnu,musl)/$(KATA_AGENT_BUILD_TYPE)/$(@F)" "$@"

$(STATIC_LIB):
	$(STATIC_LIB_BUILDER) $(STATIC_LIB_DIR)/kata-libseccomp $(STATIC_LIB_DIR)/kata-gperf

# Skoepo package packages are available in RHEL/CentOS 8 or later and Ubuntu 20.10 or later
$(SKOPEO_SRC):
	git clone -b "v$(SKOPEO_VERSION)" "$(SKOPEO_REPO)" "$(SKOPEO_SRC)"

$(SKOPEO_SRC)/bin/skopeo: $(SKOPEO_SRC)
	cd "$(SKOPEO_SRC)" && make bin/skopeo

$(SKOPEO): $(SKOPEO_SRC)/bin/skopeo
	install -D --compare "$(SKOPEO_SRC)/bin/skopeo" "$@"

# The umoci release page only publishes amd64 binaries. https://github.com/opencontainers/umoci/releases
$(UMOCI_SRC):
	git clone -b "v$(UMOCI_VERSION)" "$(UMOCI_REPO)" "$(UMOCI_SRC)"

$(UMOCI_SRC)/umoci: $(UMOCI_SRC)
	cd "$(UMOCI_SRC)" && make

$(UMOCI): $(UMOCI_SRC)/umoci
	install -D --compare "$(UMOCI_SRC)/umoci" "$@"

$(PAUSE_SRC): $(SKOPEO_SRC)/bin/skopeo
	$(SKOPEO_SRC)/bin/skopeo --policy "$(FILES_DIR)/etc/containers/policy.json" copy "$(PAUSE_REPO):$(PAUSE_VERSION)" "oci:$(PAUSE_SRC):$(PAUSE_VERSION)"

$(PAUSE): | $(PAUSE_SRC) $(UMOCI_SRC)/umoci
	$(UMOCI_SRC)/umoci unpack --rootless --image "$(PAUSE_SRC):$(PAUSE_VERSION)" "${FILES_DIR}/$(PAUSE_BUNDLE)"

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
