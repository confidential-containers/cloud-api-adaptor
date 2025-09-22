include ../Makefile.defaults

SE_BOOT   ?= false
IS_DEBIAN := $(shell if grep -q 'ID_LIKE=debian' /etc/os-release; then echo "true"; else echo "false"; fi)

REGISTRY ?= quay.io/confidential-containers
PODVM_DISTRO ?= fedora
ifeq ($(PODVM_TAG),)
	PODVM_TAG := $(VERSIONS_HASH)
endif
PODVM_BUILDER_IMAGE ?= $(REGISTRY)/podvm-builder-$(PODVM_DISTRO)-$(DISTRO_ARCH):$(PODVM_TAG)
PODVM_BINARIES_IMAGE ?= $(REGISTRY)/podvm-binaries-$(PODVM_DISTRO)-$(DISTRO_ARCH):$(PODVM_TAG)
PODVM_IMAGE ?= $(REGISTRY)/podvm-generic-$(PODVM_DISTRO)$(if $(filter $(SE_BOOT),true),-se,)-$(DISTRO_ARCH):$(PODVM_TAG)
PODVM_CONTAINER_NAME ?= $(REGISTRY)/podvm-docker-image-$(DISTRO_ARCH)
VERIFY_PROVENANCE ?= no
MKOSI_VERSION ?= v22

_SHA := $(shell git rev-parse --short HEAD)
_SHA_DIRTY := $(shell [ -n "$$(git status --porcelain 2>/dev/null)" ] && printf -- '-dirty')
IMAGE_VERSION ?= $(_SHA)$(_SHA_DIRTY)

export IMAGE_VERSION
export MKOSI_VERSION

.DEFAULT_GOAL := all
.PHONY: all
all: binaries image

.PHONY: debug
debug:binaries image-debug

.PHONY: sftp
sftp:binaries image-sftp

.PHONY: container
container: binaries image-container

define run_mkosi_in_container
	docker run --rm --privileged \
		-v "$(shell pwd)":/workspace \
		-w /workspace \
		fedora:41 \
		bash -c "dnf install -y bubblewrap coreutils git python3; mkdir -p /build; git clone -b $(MKOSI_VERSION) https://github.com/systemd/mkosi /build/mkosi; export PATH="/build/mkosi/bin:$$PATH"; mkosi --tools-tree=default --tools-tree-release=41 $(1)"
endef

binaries:
	@echo "Building binaries..."
	rm -rf ./resources/binaries-tree
ifeq ($(IS_DEBIAN),true)
	docker buildx use default
endif
	docker buildx build \
		-t $(PODVM_BINARIES_IMAGE) \
		--progress=plain \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg ARCH=$(ARCH) \
		--build-arg GO_ARCH=$(GO_ARCH) \
		--build-arg DISTRO_ARCH=$(DISTRO_ARCH) \
		--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
		--build-arg PROTOC_ARCH=$(PROTOC_ARCH) \
		--build-arg YQ_VERSION=$(YQ_VERSION) \
		--build-arg YQ_CHECKSUM=$(YQ_CHECKSUM_$(YQ_ARCH)) \
		--build-arg YQ_ARCH=$(YQ_ARCH) \
		--build-arg ORAS_VERSION=$(ORAS_VERSION) \
		--build-arg TEE_PLATFORM=$(TEE_PLATFORM) \
		--build-arg PAUSE_REPO=$(PAUSE_REPO) \
		--build-arg PAUSE_VERSION=$(PAUSE_VERSION) \
		--build-arg PAUSE_BIN=$(PAUSE_BIN) \
		--build-arg IMAGE_NAME=mkosi-podvm-binaries \
		--build-arg VERIFY_PROVENANCE=$(VERIFY_PROVENANCE) \
		$(if $(AUTHFILE),--build-arg AUTHFILE=$(AUTHFILE),) \
		$(if $(DEFAULT_AGENT_POLICY_FILE),--build-arg DEFAULT_AGENT_POLICY_FILE=$(DEFAULT_AGENT_POLICY_FILE),) \
		$(if $(filter $(PUSH),true),,-o type=local,dest="./resources/binaries-tree") \
		$(DOCKER_OPTS) \
		-f ../podvm/Dockerfile.podvm_binaries.fedora ../../

PHONY: insecure-builder
insecure-builder:
	docker buildx ls | grep "^insecure-builder" || docker buildx create \
		--driver-opt image=moby/buildkit:master \
		--name insecure-builder \
		--buildkitd-flags '--allow-insecure-entitlement security.insecure'

PHONY: image
image: insecure-builder
	rm -rf ./build
	@echo "Building image..."
ifeq ($(SE_BOOT),true)
	$(call run_mkosi_in_container,--profile production --image system)
	sudo -E ../hack/build-s390x-se-image.sh
else ifeq ($(ARCH),s390x)
	$(call run_mkosi_in_container,--profile production --image system)
	sudo -E ../hack/build-s390x-image.sh
else
	mkdir -p build
	docker buildx build \
		-f Dockerfile.mkosi \
		-t $(PODVM_IMAGE) \
		--build-arg PROFILE=production \
		--build-arg IMAGE_VERSION \
		--build-arg MKOSI_VERSION \
		--progress=plain \
		-o type=local,dest=./build \
		--builder insecure-builder \
		--allow security.insecure \
		.
	qemu-img convert -f raw -O qcow2 build/system.raw build/podvm-$(PODVM_DISTRO)-$(DISTRO_ARCH).qcow2
endif

PHONY: image-debug
image-debug: insecure-builder
	rm -rf ./build
	@echo "Building debug image..."
ifeq ($(SE_BOOT),true)
	$(call run_mkosi_in_container,--profile debug)
	sudo -E ../hack/build-s390x-se-image.sh
else ifeq ($(ARCH),s390x)
	$(call run_mkosi_in_container,--profile debug)
	sudo -E ../hack/build-s390x-image.sh
else

	mkdir -p build
	docker buildx build \
		-f Dockerfile.mkosi \
		-t $(PODVM_IMAGE) \
		--build-arg PROFILE=debug \
		--build-arg IMAGE_VERSION \
		--build-arg MKOSI_VERSION \
		--progress=plain \
		-o type=local,dest=./build \
		--builder insecure-builder \
		--allow security.insecure \
		.
	qemu-img convert -f raw -O qcow2 build/system.raw build/podvm-$(PODVM_DISTRO)-$(DISTRO_ARCH).qcow2
endif

PHONY: image-sftp
image-sftp: insecure-builder
	rm -rf ./build
	@echo "Building sftp image..."
ifeq ($(SE_BOOT),true)
	$(call run_mkosi_in_container,--profile sftp)
	sudo -E ../hack/build-s390x-se-image.sh
else ifeq ($(ARCH),s390x)
	$(call run_mkosi_in_container,--profile sftp)
	sudo -E ../hack/build-s390x-image.sh
else
	mkdir -p build
	docker buildx build \
		-f Dockerfile.mkosi \
		-t $(PODVM_IMAGE) \
		--build-arg PROFILE=sftp \
		--build-arg IMAGE_VERSION \
		--build-arg MKOSI_VERSION \
		--progress=plain \
		-o type=local,dest=./build \
		--builder insecure-builder \
		--allow security.insecure \
		.
	qemu-img convert -f raw -O qcow2 build/system.raw build/podvm-$(PODVM_DISTRO)-$(DISTRO_ARCH).qcow2
endif

PHONY: image-container
image-container:
	@echo "Building podvm container image..."
	docker buildx build \
		-t $(PODVM_CONTAINER_NAME):$(PODVM_TAG) \
		-t $(PODVM_CONTAINER_NAME):latest \
		-f Dockerfile.podvm_docker_provider .

PHONY: push-image
push-image:
	@echo "Push podvm image to remote repository..."
ifeq ($(SE_BOOT),true)
	docker push $(PODVM_NAME)-se:$(PODVM_TAG)
	docker push $(PODVM_NAME)-se:latest
else
	docker push $(PODVM_NAME):$(PODVM_TAG)
	docker push $(PODVM_NAME):latest
endif

PHONY: push-image-container
push-image-container:
	@echo "Push podvm container image to remote repository..."
	docker push $(PODVM_CONTAINER_NAME):$(PODVM_TAG)
	docker push $(PODVM_CONTAINER_NAME):latest

PHONY: clean
clean:
	rm -rf ./build
	rm -rf ./resources/binaries-tree
