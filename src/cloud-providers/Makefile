.PHONY: all build test clean

all: build test clean

build:
	@echo "→ Building all modules..."
	go build ./...

test:
	@echo "→ Running unit tests in src/cloud-providers..."
	go test ./... -coverprofile cover.out -v

clean:
	@echo "→ Cleaning build cache and test artifacts..."
	go clean -cache -testcache -modcache
	@rm -f cover.out
