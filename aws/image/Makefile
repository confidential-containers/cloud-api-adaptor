#
# SPDX-License-Identifier: Apache-2.0
#

.PHONY: build clean

UBUNTU_RELEASE     = focal
SKOPEO_VERSION     = 1.5.0
UMOCI_VERSION      = 0.4.7

IMAGE_PREFIX := podvm
UUID = $(shell cat /proc/sys/kernel/random/uuid | cut -c 1-4)

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
SKOPEO        = $(FILES_DIR)/usr/bin/skopeo
UMOCI         = $(FILES_DIR)/usr/local/bin/umoci
BINARIES = $(AGENT_PROTOCOL_FORWARDER) $(KATA_AGENT) $(SKOPEO) $(UMOCI)

AGENT_PROTOCOL_FORWARDER_SRC = ../..

KATA_AGENT_SRC = ../../../kata-containers/src/agent
KATA_AGENT_BUILD_TYPE = release

SKOPEO_SRC  = skopeo
SKOPEO_REPO = https://github.com/containers/skopeo

UMOCI_SRC   = umoci
UMOCI_REPO  = https://github.com/opencontainers/umoci

build: $(IMAGE_FILE)

$(IMAGE_FILE): $(BINARIES) $(FILES)
	mkdir -p toupload
	packer build -var vpc_id=${VPC_ID} \
		-var subnet_id=${SUBNET_ID} \
		-var account_id=${AWS_ACCOUNT_ID} \
		-var region=${AWS_REGION} \
		-var instance_type=${INSTANCE_TYPE} \
		-var ami_name=peer-pods-ami-${UUID} .
	rm -fr toupload

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

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)"

.PHONY: force
force:
