#
# SPDX-License-Identifier: Apache-2.0
#

SKOPEO_SRC =
SKOPEO_BIN = /usr/bin/skopeo

export FORWARDER_PORT ?=

ROOT_DIR := $(dir $(lastword $(MAKEFILE_LIST)))../../
include $(ROOT_DIR)podvm/Makefile.inc

export PATH := $(PATH):/usr/local/go/bin:$(HOME)/.cargo/bin

.PHONY: build clean

prerequisites: ## Install dependencies
	./prereq.sh

build: prerequisites $(BINARIES)
	./copy-files.sh

clean:
	rm -f $(BINARIES)
	rm -rf "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"
	yum remove -y skopeo

.PHONY: force
force:
