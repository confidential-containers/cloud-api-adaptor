#
# SPDX-License-Identifier: Apache-2.0
#

include ../../podvm/Makefile.inc

.PHONY: image clean

image: $(IMAGE_FILE)

$(IMAGE_FILE): $(BINARIES) $(FILES) packer-build

# When doing a packer build we already have dependencies built. We don't need to build things like kata-agent, etc.
image-build-in-docker: packer-build

packer-build:
	mkdir -p toupload
	packer build -var-file=./${PODVM_DISTRO}/variables.pkrvars.hcl ./${PODVM_DISTRO}/
	rm -fr toupload

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
