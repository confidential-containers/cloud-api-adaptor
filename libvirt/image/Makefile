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
IMAGE_FILE := $(IMAGE_NAME).qcow2

UBUNTU_IMAGE_URL  := https://cloud-images.ubuntu.com/$(UBUNTU_RELEASE)/current/$(UBUNTU_RELEASE)-server-cloudimg-$(ARCH).img
UBUNTU_IMAGE_CHECKSUM := $(shell curl -LO  https://cloud-images.ubuntu.com/"$(UBUNTU_RELEASE)"/current/SHA256SUMS && \
				grep "$(UBUNTU_RELEASE)"-server-cloudimg-"$(ARCH)".img SHA256SUMS | awk '{print $$1}' && \
				rm -f SHA256SUMS)

AGENT_PROTOCOL_FORWARDER = $(FILES_DIR)/usr/local/bin/agent-protocol-forwarder
KATA_AGENT    = $(FILES_DIR)/usr/local/bin/kata-agent
SKOPEO        = $(FILES_DIR)/usr/local/bin/skopeo.bin
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

build: $(IMAGE_FILE)

$(IMAGE_FILE): $(BINARIES) $(FILES)
	echo ${UBUNTU_IMAGE_CHECKSUM}
	rm -fr output
	rm -f cloud-init.img
	cloud-localds cloud-init.img userdata.cfg
	mkdir -p toupload
	packer build -var qemu_image_name=${IMAGE_FILE} \
		-var cloud_image_url=${UBUNTU_IMAGE_URL} \
		-var cloud_image_checksum=${UBUNTU_IMAGE_CHECKSUM} .
	rm -fr toupload
	rm -f cloud-init.img

$(AGENT_PROTOCOL_FORWARDER): force
	cd "$(AGENT_PROTOCOL_FORWARDER_SRC)" && $(MAKE) agent-protocol-forwarder
	install -D --compare "$(AGENT_PROTOCOL_FORWARDER_SRC)/agent-protocol-forwarder" "$@"

$(KATA_AGENT): force
	cd "$(KATA_AGENT_SRC)" && $(MAKE) BUILD_TYPE=$(KATA_AGENT_BUILD_TYPE)
	install -D --compare "$(KATA_AGENT_SRC)/target/$(shell uname -m)-unknown-linux-$(if $(findstring s390x,$(shell uname -m)),gnu,musl)/$(KATA_AGENT_BUILD_TYPE)/$(@F)" "$@"

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
	$(UMOCI) unpack --rootless --image "$(PAUSE_SRC):$(PAUSE_VERSION)" "${FILES_DIR}/$(PAUSE_BUNDLE)"

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
