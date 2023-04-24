#
# SPDX-License-Identifier: Apache-2.0
#

include ../../podvm/Makefile.inc

.PHONY: image clean

image: $(IMAGE_FILE)

$(IMAGE_FILE): $(BINARIES) $(FILES)
	mkdir -p toupload
	packer build -var client_id=${AZURE_CLIENT_ID} \
		-var client_secret=${AZURE_CLIENT_SECRET} \
		-var subscription_id=${AZURE_SUBSCRIPTION_ID} \
		-var tenant_id=${AZURE_TENANT_ID} \
		-var vm_size=${VM_SIZE} \
		-var resource_group=${AZURE_RESOURCE_GROUP} \
		-var az_image_name=${IMAGE_NAME} \
		-var ssh_username=${SSH_USERNAME} \
		-var publisher=${PUBLISHER} \
		-var offer=${OFFER} \
		-var sku=${SKU} \
		-var plan_name=${PLAN_NAME} \
		-var plan_product=${PLAN_PRODUCT} \
		-var plan_publisher=${PLAN_PUBLISHER} \
		./${PODVM_DISTRO}/
	rm -fr toupload

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
