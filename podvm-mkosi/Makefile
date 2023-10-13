all: fedora-binaries-builder binaries image

PHONY: fedora-binaries-builder
fedora-binaries-builder:
	@echo "Building fedora-binaries-builder image..."
	docker buildx build \
		-t fedora-binaries-builder \
		--load \
		- < ../podvm/Dockerfile.podvm_builder.fedora

PHONY: binaries
binaries:
	docker buildx use default
	@echo "Building binaries..."
	docker buildx build \
		--build-arg BUILDER_IMG=fedora-binaries-builder \
		-o type=local,dest="./resources/binaries-tree" \
		- < ../podvm/Dockerfile.podvm_binaries.fedora

define build-image =
docker buildx use default
@echo "Building mkosi-builder..."
docker buildx build \
	-t mkosi-builder \
	--load \
	--no-cache \
	.
rm -rf ./build
mkdir -p build
@echo "Building image..."
docker run \
	--rm \
	--privileged \
	-v $(PWD)/build:/mkosi/build \
	mkosi-builder \
	mkosi
endef

PHONY: image
image:
	@echo "Enabling production preset..."
	rm -rf resources/buildDebugImage
	$(build-image)


PHONY: image-debug
image-debug:
	@echo "Enabling debug preset..."
	touch resources/buildDebugImage
	$(build-image)
