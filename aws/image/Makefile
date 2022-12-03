#
# SPDX-License-Identifier: Apache-2.0
#
include ../../podvm/Makefile.inc


.PHONY: image clean

image: $(IMAGE_FILE)

$(IMAGE_FILE): $(BINARIES) $(FILES)
	mkdir -p toupload
	packer build -var vpc_id=${VPC_ID} \
		-var subnet_id=${SUBNET_ID} \
		-var account_id=${AWS_ACCOUNT_ID} \
		-var region=${AWS_REGION} \
		-var instance_type=${INSTANCE_TYPE} \
		-var ami_name=${IMAGE_NAME} .
	rm -fr toupload

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
