#
# SPDX-License-Identifier: Apache-2.0
#
.PHONY: all check cleanconfig config build clean force
.DEFAULT_GOAL := all
SKOPEO_VERSION     = 1.5.0
UMOCI_VERSION      = 0.4.7

ARCH := $(subst x86_64,amd64,$(shell uname -m))

FILES_DIR := files
FILES      = $(shell find "$(FILES_DIR)" -type f -o -type l)

AGENT_PROTOCOL_FORWARDER = $(FILES_DIR)/usr/local/bin/agent-protocol-forwarder
KATA_AGENT    = $(FILES_DIR)/usr/local/bin/kata-agent
PAUSE         = $(FILES_DIR)/$(PAUSE_BUNDLE)/rootfs/pause
SKOPEO    = $(FILES_DIR)/usr/bin/skopeo
UMOCI     = $(FILES_DIR)/usr/local/bin/umoci

BINARIES = $(AGENT_PROTOCOL_FORWARDER) $(KATA_AGENT) $(PAUSE)

ifdef USE_SKOPEO
BINARIES += $(SKOPEO) $(UMOCI)
endif

AGENT_PROTOCOL_FORWARDER_SRC = ../..

KATA_AGENT_SRC = ../../../kata-containers/src/agent
KATA_AGENT_BUILD_TYPE = release

SKOPEO_SRC  = skopeo
SKOPEO_REPO = https://github.com/containers/skopeo

UMOCI_SRC   = umoci
UMOCI_REPO  = https://github.com/opencontainers/umoci

# Embed the pause container image
# https://github.com/arronwy/kata-containers/commit/75b9f3fa3caaae62f49b4733f65cbab0cc87dbee
PAUSE_SRC     = pause
PAUSE_REPO    = docker://k8s.gcr.io/pause
PAUSE_VERSION = 3.6
PAUSE_BUNDLE  = pause_bundle

# Static libseccomp is necessary for kata-agent
# https://github.com/kata-containers/kata-containers/issues/5044#issuecomment-1239773921
STATIC_LIB_BUILDER = ../../../kata-containers/ci/install_libseccomp.sh
STATIC_LIB_DIR     = $(abspath staticlib)
STATIC_LIB         = $(STATIC_LIB_DIR)/kata-libseccomp/lib/libseccomp.a

GUESTCONFIG = settings.auto.pkrvars.hcl
VCENTERCONFIG = vcenter.auto.pkrvars.hcl
OVERWRITE =
ifndef PODVM_DISTRO
PODVM_DISTRO = ubuntu
endif

check:
ifeq (, $(shell which packer))
	$(error "packer executable is not found, I looked here: $(PATH)")
endif
ifeq (, $(shell which go))
	$(error "go executable is not found, I looked here: $(PATH)")
endif
	$(if $(strip $(GOPATH)),,$(error "GOPATH is unset, try "export GOPATH=/path/to/go""))
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


config: | check
	./${OPTS}/scripts/createconfig.sh "guest" "${OPTS}/$(GUESTCONFIG)"
	./${OPTS}/scripts/createconfig.sh "vcenter" "${OPTS}/$(VCENTERCONFIG)"

build: | config $(BINARIES) $(FILES)
	rm -rf files.tar
	cd files && tar cf ../files.tar *
	packer build ${OVERWRITE} ${OPTS}

$(AGENT_PROTOCOL_FORWARDER): force
	cd "$(AGENT_PROTOCOL_FORWARDER_SRC)" && CLOUD_PROVIDER=vsphere $(MAKE) agent-protocol-forwarder
	install -D --compare "$(AGENT_PROTOCOL_FORWARDER_SRC)/agent-protocol-forwarder" "$@"

$(KATA_AGENT): force $(STATIC_LIB)
	cd "$(KATA_AGENT_SRC)/../libs" && $(MAKE) BUILD_TYPE=$(KATA_AGENT_BUILD_TYPE) LIBSECCOMP_LINK_TYPE=static LIBSECCOMP_LIB_PATH=$(dir $(STATIC_LIB))
	cd "$(KATA_AGENT_SRC)" && $(MAKE) BUILD_TYPE=$(KATA_AGENT_BUILD_TYPE) LIBSECCOMP_LINK_TYPE=static LIBSECCOMP_LIB_PATH=$(dir $(STATIC_LIB))
	install -D --compare "$(KATA_AGENT_SRC)/target/$(shell uname -m)-unknown-linux-$(if $(findstring s390x,$(shell uname -m)),gnu,musl)/$(KATA_AGENT_BUILD_TYPE)/$(@F)" "$@"

$(STATIC_LIB):
	$(STATIC_LIB_BUILDER) $(STATIC_LIB_DIR)/kata-libseccomp $(STATIC_LIB_DIR)/kata-gperf

# Skoepo package packages are available in RHEL/CentOS 8 or later and Ubuntu 20.10 or later
$(SKOPEO_SRC):
	git clone -b "v$(SKOPEO_VERSION)" "$(SKOPEO_REPO)" "$(SKOPEO_SRC)"

$(SKOPEO_SRC)/bin/skopeo: $(SKOPEO_SRC)
	cd "$(SKOPEO_SRC)" && make bin/skopeo

$(SKOPEO): $(SKOPEO_SRC)/bin/skopeo
	install -D --compare "$(SKOPEO_SRC)/bin/skopeo" "$@"

# The umoci release page only publishes amd64 binaries. https://github.com/opencontainers/umoci/releases
$(UMOCI_SRC):
	git clone -b "v$(UMOCI_VERSION)" "$(UMOCI_REPO)" "$(UMOCI_SRC)"

$(UMOCI_SRC)/umoci: $(UMOCI_SRC)
	cd "$(UMOCI_SRC)" && make

$(UMOCI): $(UMOCI_SRC)/umoci
	install -D --compare "$(UMOCI_SRC)/umoci" "$@"

$(PAUSE_SRC): $(SKOPEO_SRC)/bin/skopeo
	$(SKOPEO_SRC)/bin/skopeo --policy "$(FILES_DIR)/etc/containers/policy.json" copy "$(PAUSE_REPO):$(PAUSE_VERSION)" "oci:$(PAUSE_SRC):$(PAUSE_VERSION)"

$(PAUSE): | $(PAUSE_SRC) $(UMOCI_SRC)/umoci
	$(UMOCI_SRC)/umoci unpack --rootless --image "$(PAUSE_SRC):$(PAUSE_VERSION)" "${FILES_DIR}/$(PAUSE_BUNDLE)"

clean:	cleanconfig
	rm -f $(BINARIES)
	rm -rf staticlib
	rm -rf files.tar
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

all: build

force:
