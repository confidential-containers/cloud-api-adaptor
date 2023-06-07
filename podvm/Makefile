
include Makefile.defaults

DOCKER_BUILDER_NAME ?= podvm_builder
DOCKER_BINARIES_NAME ?= podvm_binaries
DOCKER_PODVM_NAME ?= podvm
TARGET_ARCH ?= amd64

docker-builder-% : SOURCE_FROM = $(firstword $(subst -, ,$*))
docker-builder-% : PODVM_DISTRO = $(lastword $(subst -, ,$*))
docker-builder-%:
	docker build -t $(DOCKER_BUILDER_NAME) \
		--build-arg PODVM_DISTRO=$(PODVM_DISTRO) \
		--build-arg SOURCE_FROM=$(SOURCE_FROM) \
		--build-arg CAA_SRC=$(CAA_SRC) \
		--build-arg CAA_SRC_REF=$(CAA_SRC_REF) \
		--build-arg KATA_SRC=$(KATA_SRC) \
		--build-arg KATA_SRC_BRANCH=$(KATA_SRC_REF) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		-f Dockerfile.podvm_builder .

docker-binaries-%: docker-builder-%
	docker build -t $(DOCKER_BINARIES_NAME) \
		--build-arg BUILDER_IMG=$(DOCKER_BUILDER_NAME) \
		-f Dockerfile.podvm_binaries .

docker-builder-% : PODVM_DISTRO = $(lastword $(subst -, ,$*))
# docker-podvm-(local/remote)-(ubuntu/rhel/centos)
docker-podvm-%: docker-binaries-%
	docker build -t $(DOCKER_PODVM_NAME) \
		--build-arg BUILDER_IMG=$(DOCKER_BUILDER_NAME) \
		--build-arg BINARIES_IMG=$(DOCKER_BINARIES_NAME) \
		--build-arg IMAGE_URL=$($(PODVM_DISTRO)_$(TARGET_ARCH)_IMAGE_URL) \
		--build-arg IMAGE_CHECKSUM=$($(PODVM_DISTRO)_$(TARGET_ARCH)_IMAGE_CHECKSUM) \
		-f Dockerfile.podvm .

image: 
	$(MAKE) -f Makefile.image image
