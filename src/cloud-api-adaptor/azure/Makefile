# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

VHD_NAME = "podvm-$(subst .,_,$(IMAGE_VERSION)).vhd"

.PHONY: check-tools publish-image remove-image clean

REQUIRED_TOOLS = \
	az \
	jq \
	oras \
	qemu-img

check-tools:
	@$(foreach tool,$(REQUIRED_TOOLS),command -v $(tool) >/dev/null 2>&1 \
		|| { echo "error: $(tool) not found"; exit 1; };)

podvm.tar.xz:
	@if [ -z "$(PODVM_OCI_IMAGE)" ]; then echo "error: PODVM_OCI_IMAGE is unset"; exit 1; fi
	@oras pull --no-tty $(PODVM_OCI_IMAGE) > /dev/null

# qcow2 file is older than the tarball, hence we need to touch it
podvm-ubuntu-amd64.qcow2: podvm.tar.xz
	@tar xf $< && touch $@

image.raw: podvm-ubuntu-amd64.qcow2
	@qemu-img convert -f qcow2 -O raw $< $@

resized.raw: image.raw
	@size="$$(qemu-img info -f raw --output json $< | jq '.["virtual-size"]')" && \
	mb="$$((1024 * 1024))" && \
	rounded_size="$$((($$size/$$mb + 1) * $$mb))" && \
	cp $< $@ && \
	qemu-img resize -q -f raw $@ "$$rounded_size"

image.vhd: resized.raw
	@qemu-img convert -q -f raw -o subformat=fixed,force_size -O vpc $< $@

REQUIRED_SIG_VARS := \
	CONTAINER \
	IMAGE_DEFINITION \
	IMAGE_GALLERY \
	IMAGE_VERSION \
	RESOURCE_GROUP \
	STORAGE_ACCOUNT

# Note: this uses shallow replication, which is only suited for e2e testing.
publish-image: image.vhd
	@$(foreach var,$(REQUIRED_SIG_VARS), \
		if [ -z "$($(var))" ]; then echo "error: $(var) is unset"; exit 1; fi; )
	@az storage blob upload \
		--account-name "$(STORAGE_ACCOUNT)" \
		--container-name "$(CONTAINER)" \
		--file "$<" \
		--name "$(VHD_NAME)" \
		--only-show-errors \
		--no-progress -o none && \
	storage_account_id="$$(az storage account show \
		--name "$(STORAGE_ACCOUNT)" \
		--resource-group "$(RESOURCE_GROUP)" \
		-o tsv --query id)" && \
	blob_url="$$(az storage blob url \
		--account-name "$(STORAGE_ACCOUNT)" \
		--container-name "$(CONTAINER)" \
		--name "$(VHD_NAME)" \
		-o tsv)" && \
	az sig image-version create \
		--resource-group "$(RESOURCE_GROUP)" \
		--gallery-name "$(IMAGE_GALLERY)" \
		--gallery-image-definition "$(IMAGE_DEFINITION)" \
		--gallery-image-version "$(IMAGE_VERSION)" \
		--os-vhd-storage-account "$$storage_account_id" \
		--os-vhd-uri "$$blob_url" \
		--replication-mode Shallow \
		--only-show-errors \
		-o tsv --query id

remove-image:
	@$(foreach var,$(REQUIRED_SIG_VARS), \
		if [ -z "$($(var))" ]; then echo "error: $(var) is unset"; exit 1; fi; )
	@az sig image-version delete \
		--resource-group "$(RESOURCE_GROUP)" \
		--gallery-name "$(IMAGE_GALLERY)" \
		--gallery-image-definition "$(IMAGE_DEFINITION)" \
		--gallery-image-version "$(IMAGE_VERSION)" \
		--no-wait \
		--only-show-errors \
		-o none && \
	az storage blob delete \
		--account-name "$(STORAGE_ACCOUNT)" \
		--container-name "$(CONTAINER)" \
		--name "$(VHD_NAME)" \
		--only-show-errors \
		-o none

clean:
	rm -f podvm.tar.xz podvm-ubuntu-amd64.qcow2 image.raw resized.raw image.vhd
