#
# SPDX-License-Identifier: Apache-2.0
#
.PHONY: all cleanconfig config build clean force
.DEFAULT_GOAL := all


GUESTCONFIG = settings.auto.pkrvars.hcl
VCENTERCONFIG = vcenter.auto.pkrvars.hcl
DIR = $(shell pwd)
OVERWRITE =
ifndef PODVM_DISTRO
PODVM_DISTRO = ubuntu
endif

ifeq ($(FORCE),1)
    $(eval OVERWRITE := -force)
endif
ifeq ($(PODVM_DISTRO),ubuntu)
    $(eval OPTS := ubuntu)
else ifeq ($(PODVM_DISTRO),rhel)
    $(eval OPTS := rhel)
else
    $(error PODVM_DISTRO is invalid or not defined)
endif

cleanconfig:
ifeq ($(PODVM_DISTRO),ubuntu)
	$(eval OPTS := ubuntu)
else ifeq ($(PODVM_DISTRO),rhel)
	$(eval OPTS := rhel)
else
	$(error PODVM_DISTRO is invalid or not defined)
endif
	rm -rf ${OPTS}/$(VCENTERCONFIG) ${OPTS}/$(GUESTCONFIG)


config:
	./${OPTS}/scripts/createconfig.sh "guest" "${OPTS}/$(GUESTCONFIG)"
	./${OPTS}/scripts/createconfig.sh "vcenter" "${OPTS}/$(VCENTERCONFIG)"
# Ensure the vcenter credentials user input target is run first
include ../../podvm/Makefile.inc

build: | config $(BINARIES) $(FILES)
	rm -rf files.tar
	cd ../../podvm/files && tar cf ${DIR}/files.tar *
	cd files && tar uf ${DIR}/files.tar *
	packer build ${OVERWRITE} ${OPTS}
	rm -rf files.tar

clean:	cleanconfig
	rm -f $(BINARIES)
	rm -rf files.tar
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

all: build

force:
