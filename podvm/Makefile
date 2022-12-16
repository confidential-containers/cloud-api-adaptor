#
# SPDX-License-Identifier: Apache-2.0
#

include Makefile.inc

.PHONY: image clean

UBUNTU_IMAGE_URL  ?= https://cloud-images.ubuntu.com/$(UBUNTU_RELEASE)/current/$(UBUNTU_RELEASE)-server-cloudimg-$(ARCH).img
UBUNTU_IMAGE_CHECKSUM ?= $(shell curl -LO  https://cloud-images.ubuntu.com/"$(UBUNTU_RELEASE)"/current/SHA256SUMS && \
				grep "$(UBUNTU_RELEASE)"-server-cloudimg-"$(ARCH)".img SHA256SUMS | awk '{print $$1}' && \
				rm -f SHA256SUMS)

IMAGE_SUFFIX := .qcow2
PODVM_DISTRO ?= ubuntu
KATA_AGENT_SRC := ../../kata-containers/src/agent
STATIC_LIBSECCOMP_BUILDER := ../../kata-containers/ci/install_libseccomp.sh
AGENT_PROTOCOL_FORWARDER_SRC := ../

image: $(IMAGE_FILE)

setopts:
ifeq ($(PODVM_DISTRO),ubuntu)
	@echo defined
	$(eval OPTS := -var qemu_image_name=${IMAGE_FILE} \
	-var cloud_image_url=${UBUNTU_IMAGE_URL} \
	-var cloud_image_checksum=${UBUNTU_IMAGE_CHECKSUM} qcow2/ubuntu)
else ifeq ($(PODVM_DISTRO),rhel)
ifndef RHEL_IMAGE_URL
	$(error "RHEL_IMAGE_URL is not defined")
endif
ifndef RHEL_IMAGE_CHECKSUM
	$(error "RHEL_IMAGE_CHECKSUM is not defined")
endif
	$(eval OPTS := -var qemu_image_name=${IMAGE_FILE} \
	-var cloud_image_url=${RHEL_IMAGE_URL} \
	-var disk_size=11144 \
	-var ssh_username=cloud-user \
	-var cloud_image_checksum=${RHEL_IMAGE_CHECKSUM} qcow2/rhel)

else ifeq ($(PODVM_DISTRO),centos)
ifndef CENTOS_IMAGE_URL
	$(error "CENTOS_IMAGE_URL is not defined")
endif
ifndef CENTOS_IMAGE_CHECKSUM
	$(error "CENTOS_IMAGE_CHECKSUM is not defined")
endif
	$(eval OPTS := -var qemu_image_name=${IMAGE_FILE} \
	-var cloud_image_url=${CENTOS_IMAGE_URL} \
	-var cloud_image_checksum=${CENTOS_IMAGE_CHECKSUM} qcow2/centos)

else
	$(error PODVM_DISTRO is invalid or not defined)
endif

$(IMAGE_FILE): $(BINARIES) $(FILES) setopts
	rm -fr output
	rm -f cloud-init.img
	cloud-localds cloud-init.img qcow2/userdata.cfg
	mkdir -p toupload
	packer build ${OPTS}
	rm -fr toupload
	rm -f cloud-init.img

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
