#
# SPDX-License-Identifier: Apache-2.0
#

include ../../podvm/Makefile.inc

.PHONY: image clean

image: $(IMAGE_FILE)

$(IMAGE_FILE): $(BINARIES) $(FILES)
	mkdir -p toupload
	# export the required packer variables as PKR_VAR_<var_name>
	# export PKR_VAR_client_id="${AZURE_CLIENT_ID}"
	# export PKR_VAR_client_secret="${AZURE_CLIENT_SECRET:%=REDACTED}"
	# export PKR_VAR_subscription_id="${AZURE_SUBSCRIPTION_ID}"
	# export PKR_VAR_tenant_id="${AZURE_TENANT_ID}"
	# export PKR_VAR_resource_group="${AZURE_RESOURCE_GROUP}"
	# export PKR_VAR_az_image_name="${IMAGE_NAME}"
	# export PKR_VAR_vm_size="${VM_SIZE}"
	# export PKR_VAR_ssh_username="${SSH_USERNAME}"
	# export PKR_VAR_az_gallery_name="${GALLERY_NAME}"
	# export PKR_VAR_az_gallery_image_name="${GALLERY_IMAGE_NAME}"
	# export PKR_VAR_az_gallery_image_version="${GALLERY_IMAGE_VERSION}"

	packer build ./${PODVM_DISTRO}/
	rm -fr toupload

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
