AA_KBC  ?= offline_fs_kbc
ARCH    ?= $(subst x86_64,amd64,$(shell uname -m))
BUILDER = fedora-binaries-builder-$(ARCH)

.DEFAULT_GOAL := all
.PHONY: all
all: fedora-binaries-builder binaries image

.PHONY: debug
debug: fedora-binaries-builder binaries image-debug

PHONY: fedora-binaries-builder
fedora-binaries-builder:
	@echo "Building $(BUILDER) image..."
ifeq ($(ARCH),s390x)
	docker buildx build \
		-t $(BUILDER) \
		--build-arg ARCH=s390x \
		--build-arg PROTOC_ARCH=s390x \
		--build-arg YQ_ARCH=s390x \
		--build-arg YQ_CHECKSUM=sha256:4e6324d08630e7df733894a11830412a43703682d65a76f1fc925aac08268a45 \
		--load \
		-f ../podvm/Dockerfile.podvm_builder.fedora ../.
else
	docker buildx build \
		-t $(BUILDER) \
		--load \
		-f ../podvm/Dockerfile.podvm_builder.fedora ../.
endif

PHONY: binaries
binaries:
	docker buildx use default
	@echo "Building binaries..."
	rm -rf ./resources/binaries-tree
	cp -rf ../../../.git ../../.git
	docker buildx build \
		--build-arg BUILDER_IMG=$(BUILDER) \
		--build-arg AA_KBC=$(AA_KBC) \
		$(if $(DEFAULT_AGENT_POLICY_FILE),--build-arg DEFAULT_AGENT_POLICY_FILE=$(DEFAULT_AGENT_POLICY_FILE),) \
		-o type=local,dest="./resources/binaries-tree" \
		-f ../podvm/Dockerfile.podvm_binaries.fedora ../../
	rm -rf ../../.git

PHONY: image
image:
	@echo "Enabling production preset..."
	rm -rf resources/build*Image
	rm -rf ./build
	@echo "Building image..."
ifeq ($(ARCH),s390x)
	touch resources/buildS390xImage
	mkosi --profile production.conf
	../hack/build-s390x-image.sh
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
ifeq ($(ARCH),s390x)
	touch resources/buildS390xImage
	mkosi --profile debug.conf
	../hack/build-s390x-image.sh
else
	touch resources/buildBootableImage
	nix develop ..#podvm-mkosi --command mkosi --environment=VARIANT_ID=debug
endif

PHONY: clean
clean:
	rm -rf resources/buildDebugImage
	rm -rf ./build
	rm -rf ./resources/binaries-tree
