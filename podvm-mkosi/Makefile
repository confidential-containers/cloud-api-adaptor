AA_KBC ?= offline_fs_kbc

.DEFAULT_GOAL := all
.PHONY: all
all: fedora-binaries-builder binaries image

.PHONY: debug
debug: fedora-binaries-builder binaries image-debug

PHONY: fedora-binaries-builder
fedora-binaries-builder:
	@echo "Building fedora-binaries-builder image..."
	docker buildx build \
		-t fedora-binaries-builder \
		--load \
		-f ../podvm/Dockerfile.podvm_builder.fedora ../.

PHONY: binaries
binaries:
	docker buildx use default
	@echo "Building binaries..."
	rm -rf ./resources/binaries-tree
	docker buildx build \
		--build-arg BUILDER_IMG=fedora-binaries-builder \
		--build-arg AA_KBC=$(AA_KBC) \
		-o type=local,dest="./resources/binaries-tree" \
		-f ../podvm/Dockerfile.podvm_binaries.fedora ../.

PHONY: image
image:
	@echo "Enabling production preset..."
	rm -rf resources/buildDebugImage
	rm -rf ./build
	@echo "Building image..."
	nix develop ..#podvm-mkosi --command mkosi --environment=VARIANT_ID=production

PHONY: image-debug
image-debug:
	@echo "Enabling debug preset..."
	touch resources/buildDebugImage
	rm -rf ./build
	@echo "Building debug image..."
	nix develop ..#podvm-mkosi --command mkosi --environment=VARIANT_ID=debug

PHONY: clean
clean:
	rm -rf resources/buildDebugImage
	rm -rf ./build
	rm -rf ./resources/binaries-tree
