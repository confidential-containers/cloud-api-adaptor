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

PHONY: image
image: image-builder image-build

PHONY: image-builder
image-builder:
	docker buildx use default
	@echo "Building mkosi-builder..."
	docker buildx build \
		-t mkosi-builder \
		--load \
		--no-cache \
		.

PHONY: image-build
image-build:
	rm -rf ./build
	mkdir -p build
	@echo "Building image..."
	docker run \
		--rm \
		--privileged \
		-v $(PWD)/build:/mkosi/build \
		mkosi-builder \
		mkosi
