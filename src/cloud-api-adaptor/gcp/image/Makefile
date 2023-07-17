#
# SPDX-License-Identifier: Apache-2.0
#
include ../../podvm/Makefile.inc


.PHONY: image clean

GCP_MACHINE_TYPE ?= e2-medium
GCP_NETWORK ?= default

image: $(IMAGE_FILE)

$(IMAGE_FILE): setopts $(BINARIES) $(FILES)
	mkdir -p toupload
	packer build ${OPTS} \
		-var zone=${GCP_ZONE} \
		-var machine_type=${GCP_MACHINE_TYPE} \
		-var network=${GCP_NETWORK} \
		-var gce_image_name=${IMAGE_NAME} ./${PODVM_DISTRO}/
	rm -fr toupload

setopts:
ifeq ($(PODVM_DISTRO),)
	$(error PODVM_DISTRO is not defined)
endif
ifeq ($(GCP_ZONE),)
	$(error GCP_ZONE is not defined)
endif

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
