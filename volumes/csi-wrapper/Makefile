BINARIES     := csi-controller-wrapper csi-node-wrapper csi-podvm-wrapper
SOURCEDIRS := ./cmd ./pkg
PACKAGES   := $(shell go list $(addsuffix /...,$(SOURCEDIRS)))
SOURCES    := $(shell find $(SOURCEDIRS) -name '*.go' -print)

# Determine the go arch (amd64 for x86, ppc64le for power, s390x for z)
GO_BLD_ARCH=$(shell go env GOHOSTARCH)
BUILD_ROOT=${PWD}

all: build
build: $(BINARIES)

$(BINARIES): $(SOURCES)
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GO_BLD_ARCH) go build  -o "$(BUILD_ROOT)/build/$@/$@" "cmd/$@/main.go"

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
csi-node-wrapper-docker: csi-node-wrapper
	cp hack/dockers/csi-node-wrapper/Dockerfile $(BUILD_ROOT)/build/csi-node-wrapper/
	docker build -t csi-node-wrapper:local $(BUILD_ROOT)/build/csi-node-wrapper

.PHONY: csi-controller-wrapper-docker
csi-controller-wrapper-docker: csi-controller-wrapper
	cp hack/dockers/csi-controller-wrapper/Dockerfile $(BUILD_ROOT)/build/csi-controller-wrapper/
	docker build -t csi-controller-wrapper:local $(BUILD_ROOT)/build/csi-controller-wrapper

.PHONY: csi-podvm-wrapper-docker
csi-podvm-wrapper-docker: csi-podvm-wrapper
	cp hack/dockers/csi-podvm-wrapper/Dockerfile $(BUILD_ROOT)/build/csi-podvm-wrapper/
	docker build -t csi-podvm-wrapper:local $(BUILD_ROOT)/build/csi-podvm-wrapper

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
