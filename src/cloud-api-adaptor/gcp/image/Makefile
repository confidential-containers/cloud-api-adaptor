#
# SPDX-License-Identifier: Apache-2.0
#
#include ../../podvm/Makefile.inc


.PHONY: image clean

GCP_MACHINE_TYPE ?= e2-medium
GCP_NETWORK ?= default

PODVM_DISTRO ?= ubuntu

PODVM_BUILDER_IMAGE ?= $(REGISTRY)/podvm-builder-$(PODVM_DISTRO):$(PODVM_TAG)
PODVM_BINARIES_IMAGE ?= $(REGISTRY)/podvm-binaries-$(PODVM_DISTRO)-$(ARCH):$(PODVM_TAG)

IMAGE_FILE ?= podvm-$(or $(CLOUD_PROVIDER),gcp)-$(PODVM_DISTRO)-$(ARCH)
PODVM_IMAGE ?= $(REGISTRY)/$(IMAGE_FILE):$(PODVM_TAG)

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


podvm-image:
	docker buildx build --progress=plain --no-cache -t $(PODVM_IMAGE) -f Dockerfile \
	--build-arg BUILDER_IMG=$(PODVM_BUILDER_IMAGE) \
	--build-arg BINARIES_IMG=$(PODVM_BINARIES_IMAGE) \
	--build-arg PODVM_DISTRO=$(PODVM_DISTRO) \
	--build-arg ARCH=$(ARCH) \
	--build-arg GCP_ZONE=$(GCP_ZONE) \
	--build-arg GCP_PROJECT_ID=$(GCP_PROJECT_ID) \
	--build-arg CLOUD_PROVIDER=$(or $(CLOUD_PROVIDER),gcp) \
	--build-arg IMAGE_URL=$(IMAGE_URL) \
	--build-arg IMAGE_CHECKSUM=$(IMAGE_CHECKSUM) \
	 --secret id=APPLICATION_KEY,src=${GOOGLE_APPLICATION_CREDENTIALS} \
	$(DOCKER_OPTS) .
	rm -rf .git
clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
