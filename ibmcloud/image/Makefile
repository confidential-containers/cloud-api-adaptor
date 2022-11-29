#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

include ../../podvm/Makefile.inc

.PHONY: build push verify ubuntu clean

UBUNTU_RELEASE     = focal
UBUNTU_IMAGE_URL  := https://cloud-images.ubuntu.com/$(UBUNTU_RELEASE)/current/$(UBUNTU_RELEASE)-server-cloudimg-$(ARCH).img
UBUNTU_IMAGE_FILE := $(notdir $(UBUNTU_IMAGE_URL))

UBUNTU_PACKAGES = jq

IMAGE_SUFFIX := .qcow2

build: $(IMAGE_FILE)

push: $(IMAGE_FILE)
	./push.sh --name "$(IMAGE_NAME)" --path "$(IMAGE_FILE)"

verify: push
	./verify.sh --image "$(IMAGE_NAME)"

SUDO =
WORKDIR = .

$(IMAGE_FILE): $(UBUNTU_IMAGE_FILE) $(BINARIES) $(FILES)
	rm -f "$(IMAGE_FILE)"
	$(SUDO) ./build.sh  --root "$(FILES_DIR)" --packages "$(UBUNTU_PACKAGES)" --base "$(UBUNTU_IMAGE_FILE)" --output "$(IMAGE_FILE)"  --workdir "$(WORKDIR)"

ubuntu: $(UBUNTU_IMAGE_FILE)
$(UBUNTU_IMAGE_FILE):
	curl -s -L -o "$@" "$(UBUNTU_IMAGE_URL)"

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
