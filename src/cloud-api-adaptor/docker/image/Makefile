include ../../Makefile.defaults

ARCH    ?= $(subst x86_64,amd64,$(shell uname -m))
BUILDER = ubuntu-binaries-builder-$(ARCH)
PODVM_IMG ?= quay.io/confidential-containers/podvm-docker-image
AGENT_POLICY = no
ATTESTER ?= none

.DEFAULT_GOAL := all
.PHONY: all
all: ubuntu-binaries-builder binaries

PHONY: ubuntu-binaries-builder
ubuntu-binaries-builder:
	@echo "Building $(BUILDER) image..."
ifeq ($(ARCH),s390x)
	docker buildx build \
		-t $(BUILDER) \
		--build-arg ARCH=s390x \
		--build-arg PROTOC_ARCH=s390x_64 \
		--build-arg YQ_ARCH=s390x \
		--build-arg YQ_CHECKSUM=sha256:4e6324d08630e7df733894a11830412a43703682d65a76f1fc925aac08268a45 \
		--load \
		-f ../../podvm/Dockerfile.podvm_builder ../.
else
	docker buildx build \
		-t $(BUILDER) \
		--load \
		-f ../../podvm/Dockerfile.podvm_builder ../.
endif

PHONY: binaries
binaries:
	docker buildx use default
	@echo "Building binaries..."
	rm -rf ./resources/binaries-tree
	cp -rf ../../../../.git ../../../.git
	docker buildx build \
		--build-arg BUILDER_IMG=$(BUILDER) \
		--build-arg ATTESTER=$(ATTESTER) \
		$(if $(DEFAULT_AGENT_POLICY_FILE),--build-arg DEFAULT_AGENT_POLICY_FILE=$(DEFAULT_AGENT_POLICY_FILE),) \
		-o type=local,dest="./resources/binaries-tree" \
		-f ../../podvm/Dockerfile.podvm_binaries ../../../
	rm -rf ../../../.git

PHONY: image
image:
	@echo "Building docker podvm image..."
	docker build  --no-cache -t $(PODVM_IMG) -f Dockerfile .

PHONY: clean
clean:
	rm -rf resources/buildDebugImage
	rm -rf ./build
	rm -rf ./resources/binaries-tree
