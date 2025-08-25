GO ?= go

.PHONY: all build test

all: build test

build:
	@echo "→ Building all modules..."
	go build ./...

test:
	@echo "→ Running unit tests in src/cloud-providers..."
	go test ./... -coverprofile cover.out -v
