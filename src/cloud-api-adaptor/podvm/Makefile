#
# SPDX-License-Identifier: Apache-2.0
#

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(SELF_DIR)Makefile.inc

.PHONY: image clean clean_sources

IMAGE_SUFFIX := .qcow2
PODVM_DISTRO ?= ubuntu
IMAGE_URL := $(or $(IMAGE_URL),$($(PODVM_DISTRO)_$(DEB_ARCH)_IMAGE_URL))
IMAGE_CHECKSUM := $(or $(IMAGE_CHECKSUM),$($(PODVM_DISTRO)_$(DEB_ARCH)_IMAGE_CHECKSUM))

ifndef IMAGE_URL
$(error "IMAGE_URL is not defined")
endif
ifndef IMAGE_CHECKSUM
$(error "IMAGE_CHECKSUM is not defined")
endif

KATA_AGENT_SRC := ../../kata-containers/src/agent
STATIC_LIBSECCOMP_BUILDER := ../../kata-containers/ci/install_libseccomp.sh
AGENT_PROTOCOL_FORWARDER_SRC := ../

QEMU_MACHINE_TYPE_s390x := s390-ccw-virtio

UEFI ?= false
UEFI_FIRMWARE_LOCATION ?=

SE_BOOT ?= 0

# Env variable for default qemu builder options across distros/arch
PACKER_DEFAULT_OPTS ?= -var qemu_image_name=${IMAGE_FILE} \
	-var cloud_image_url=${IMAGE_URL} \
	-var cloud_image_checksum=${IMAGE_CHECKSUM}

image: $(IMAGE_FILE)

setopts:
ifeq ($(PODVM_DISTRO),ubuntu)
	@echo defined
	$(eval OPTS := -var se_boot=${SE_BOOT})
ifneq ($(HOST_ARCH),$(ARCH))
	$(eval OPTS += -var machine_type=${QEMU_MACHINE_TYPE_${ARCH}})
ifndef QEMU_BINARY
	$(eval OPTS += -var qemu_binary=qemu-system-${ARCH})
endif

endif
else ifeq ($(PODVM_DISTRO),rhel)
	@echo defined
	$(eval OPTS := -var disk_size=11144)
ifeq ($(ARCH),s390x)
	$(eval OPTS += -var se_boot=${SE_BOOT})
	$(eval OPTS += -var machine_type=${QEMU_MACHINE_TYPE_${ARCH}})
	$(eval OPTS += -var cpu_type=max)
	$(eval OPTS += -var os_arch=s390x)
ifndef QEMU_BINARY
	$(eval OPTS += -var qemu_binary=qemu-system-${ARCH})
endif
endif
else
	$(error PODVM_DISTRO is invalid or not defined)
endif

ifdef QEMU_BINARY
	$(eval OPTS += -var qemu_binary=${QEMU_BINARY} )
endif

# UEFI is enabled for x86_64 only
ifeq ($(UEFI),true)
	$(eval OPTS += -var is_uefi=true -var os_arch="x86_64" )
ifdef UEFI_FIRMWARE_LOCATION
	$(eval OPTS += -var uefi_firmware=${UEFI_FIRMWARE_LOCATION} )
endif
endif

$(IMAGE_FILE): $(BINARIES) $(FILES) setopts
	rm -fr output
	rm -f cloud-init.img
	cloud-localds cloud-init.img qcow2/userdata.cfg
	mkdir -p toupload
	if [ "${SE_BOOT}" = "true" ] && [ "${ARCH}" = "s390x" ]; then \
		qemu-img create -f qcow2 "se-${IMAGE_FILE}" 100G; \
		# Temporary workaround for installing cryptsetup on RHEL 9.4 and below s390x base images for enabling se \
		# Due to issue: https://gitlab.com/qemu-project/qemu/-/issues/2054 \
		# Remove this if using the latest QEMU version (v9.0.0) \
		if [ "${PODVM_DISTRO}" = "rhel" ]; then \
			yum install -y cryptsetup; \
			cp /usr/sbin/cryptsetup ./files; \
		fi \
	fi
	packer init ./qcow2/${PODVM_DISTRO}
	if [ "${ARCH}" = "x86_64" ]; then \
		packer plugins install github.com/hashicorp/qemu v1.1.0; \
	fi
	packer build ${PACKER_DEFAULT_OPTS} ${OPTS} qcow2/${PODVM_DISTRO}
	rm -fr toupload
	rm -f cloud-init.img

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"
	rm -fr "$(OPA_SRC)"


