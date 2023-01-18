#
# SPDX-License-Identifier: Apache-2.0
#

include ../../podvm/Makefile.inc

.PHONY: image clean

image: $(IMAGE_FILE)

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

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
