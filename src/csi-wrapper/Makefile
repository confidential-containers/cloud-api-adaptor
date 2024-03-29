BINARIES     := csi-controller-wrapper csi-node-wrapper csi-podvm-wrapper
SOURCEDIRS := ./cmd ./pkg
PACKAGES   := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES    := $(shell find $(SOURCEDIRS) -name '*.go' -print)

ARCH        ?= $(subst x86_64,amd64,$(shell uname -m))
GOOPTIONS   ?= GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0
BUILD_ROOT=${PWD}

all: build
build: $(BINARIES)

$(BINARIES): $(SOURCES)
	$(GOOPTIONS) go build  -o "$(BUILD_ROOT)/build/$@/$@" "cmd/$@/main.go"

test:
	go test -cover $(PACKAGES)

check: fmt vet

fmt:
	find $(SOURCEDIRS) -name '*.go' -print0 | xargs -0 gofmt -l -d

vet:
	go vet $(PACKAGES)

clean:
	rm -fr $(BUILD_ROOT)/build/*

.PHONY: csi-node-wrapper-docker
csi-node-wrapper-docker:
	cd ../ && docker buildx build --platform "linux/$(ARCH)" \
		--build-arg BINARY=csi-node-wrapper \
		--build-arg SOURCE_FROM=local \
		-t csi-node-wrapper:local \
		-f csi-wrapper/Dockerfile.csi_wrappers --load .

.PHONY: csi-controller-wrapper-docker
csi-controller-wrapper-docker:
	cd ../ && docker buildx build --platform "linux/$(ARCH)" \
		--build-arg BINARY=csi-controller-wrapper \
		--build-arg SOURCE_FROM=local \
		-t csi-controller-wrapper:local \
		-f csi-wrapper/Dockerfile.csi_wrappers --load .

.PHONY: csi-podvm-wrapper-docker
csi-podvm-wrapper-docker:
	cd ../ && docker buildx build --platform "linux/$(ARCH)" \
		--build-arg BINARY=csi-podvm-wrapper \
		--build-arg SOURCE_FROM=local \
		-t csi-podvm-wrapper:local \
		-f csi-wrapper/Dockerfile.csi_wrappers --load .

.PHONY: import-csi-node-wrapper-docker
import-csi-node-wrapper-docker: csi-node-wrapper-docker
	docker save "csi-node-wrapper:local" | ctr -n=k8s.io images import -

.PHONY: import-csi-controller-wrapper-docker
import-csi-controller-wrapper-docker: csi-controller-wrapper-docker
	docker save "csi-controller-wrapper:local" | ctr -n=k8s.io images import -

.PHONY: import-csi-podvm-wrapper-docker
import-csi-podvm-wrapper-docker: csi-podvm-wrapper-docker
	docker save "csi-podvm-wrapper:local" | ctr -n=k8s.io images import -

.PHONY: all build check fmt vet clean
