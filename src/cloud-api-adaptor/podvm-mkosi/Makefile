include ../Makefile.defaults

ATTESTER  ?= none
ARCH      ?= $(subst x86_64,amd64,$(shell uname -m))
BUILDER   = fedora-binaries-builder-$(ARCH)
SE_BOOT   ?= false
IS_DEBIAN := $(shell if grep -q 'ID_LIKE=debian' /etc/os-release; then echo "true"; else echo "false"; fi)

REGISTRY ?= quay.io/confidential-containers
PODVM_DISTRO ?= fedora
PODVM_TAG ?= $(VERSIONS_HASH)
PODVM_NAME ?= $(REGISTRY)/podvm-generic-$(PODVM_DISTRO)-$(ARCH)
PODVM_CONTAINER_NAME ?= $(REGISTRY)/podvm-docker-image

.DEFAULT_GOAL := all
.PHONY: all
all: fedora-binaries-builder binaries image

.PHONY: debug
debug: fedora-binaries-builder binaries image-debug

.PHONY: container
container: fedora-binaries-builder binaries image-container

ifeq ($(ARCH),s390x)
YQ_CHECKSUM = $(YQ_CHECKSUM_s390x)
endif

PHONY: fedora-binaries-builder
fedora-binaries-builder:
	@echo "Building $(BUILDER) image..."
	docker buildx build \
		-t $(BUILDER) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg ARCH=$(ARCH) \
		--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		--build-arg YQ_VERSION=$(YQ_VERSION) \
		--build-arg YQ_CHECKSUM=$(YQ_CHECKSUM) \
		--build-arg YQ_ARCH=$(ARCH) \
		--build-arg PROTOC_ARCH=$(if $(filter amd64,$(ARCH)),x86_64,s390x) \
		--load \
		-f ../podvm/Dockerfile.podvm_builder.fedora ../.

PHONY: binaries
binaries:
	@echo "Building binaries..."
	rm -rf ./resources/binaries-tree
ifeq ($(IS_DEBIAN),true)
	docker buildx use default
endif
	docker buildx build \
		--build-arg BUILDER_IMG=$(BUILDER) \
		--build-arg ATTESTER=$(ATTESTER) \
		--build-arg PAUSE_REPO=$(PAUSE_REPO) \
		--build-arg PAUSE_VERSION=$(PAUSE_VERSION) \
		--build-arg PAUSE_BIN=$(PAUSE_BIN) \
		--build-arg IMAGE_NAME=mkosi-podvm-binaries \
		$(if $(AUTHFILE),--build-arg AUTHFILE=$(AUTHFILE),) \
		$(if $(DEFAULT_AGENT_POLICY_FILE),--build-arg DEFAULT_AGENT_POLICY_FILE=$(DEFAULT_AGENT_POLICY_FILE),) \
		-o type=local,dest="./resources/binaries-tree" \
		-f ../podvm/Dockerfile.podvm_binaries.fedora ../../

PHONY: image
image:
	@echo "Enabling production preset..."
	rm -rf resources/build*Image
	rm -rf ./build
	@echo "Building image..."
ifeq ($(SE_BOOT),true)
	touch resources/buildS390xImage
	sudo mkosi --profile production.conf --image system
	sudo -E ../hack/build-s390x-se-image.sh
	@echo "Building docker image..."
	docker buildx build \
		-t $(PODVM_NAME)-se:$(PODVM_TAG) \
		-t $(PODVM_NAME)-se:latest \
		--load \
		-f ../podvm/Dockerfile.podvm.fedora .
else ifeq ($(ARCH),s390x)
	touch resources/buildS390xImage
	sudo mkosi --profile production.conf --image system
	sudo -E ../hack/build-s390x-image.sh
	@echo "Building docker image..."
	docker buildx build \
		-t $(PODVM_NAME):$(PODVM_TAG) \
		-t $(PODVM_NAME):latest \
		--load \
		-f ../podvm/Dockerfile.podvm.fedora .
else
	touch resources/buildBootableImage
	nix develop ..#podvm-mkosi --command mkosi --environment=VARIANT_ID=production
endif

PHONY: image-debug
image-debug:
	@echo "Enabling debug preset..."
	rm -rf resources/build*Image
	touch resources/buildDebugImage
	rm -rf ./build
	@echo "Building debug image..."
ifeq ($(SE_BOOT),true)
	touch resources/buildS390xImage
	sudo mkosi --profile debug.conf
	sudo -E ../hack/build-s390x-se-image.sh
	@echo "Building docker image..."
	docker buildx build \
		-t $(PODVM_NAME)-se:$(PODVM_TAG) \
		-t $(PODVM_NAME)-se:latest \
		--load \
		-f ../podvm/Dockerfile.podvm.fedora .
else ifeq ($(ARCH),s390x)
	touch resources/buildS390xImage
	sudo mkosi --profile debug.conf
	sudo -E ../hack/build-s390x-image.sh
	@echo "Building docker image..."
	docker buildx build \
		-t $(PODVM_NAME):$(PODVM_TAG) \
		-t $(PODVM_NAME):latest \
		--load \
		-f ../podvm/Dockerfile.podvm.fedora .
else
	touch resources/buildBootableImage
	nix develop ..#podvm-mkosi --command mkosi --environment=VARIANT_ID=debug
endif

PHONY: image-container
image-container:
	@echo "Building podvm container image..."
	docker buildx build \
		-t $(PODVM_CONTAINER_NAME):$(PODVM_TAG) \
		-t $(PODVM_CONTAINER_NAME):latest \
		-f Dockerfile.podvm .

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
	rm -rf resources/buildDebugImage
	rm -rf ./build
	rm -rf ./resources/binaries-tree
