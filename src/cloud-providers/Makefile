.PHONY: all build test clean config-extractor

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
	@rm -rf bin/

config-extractor: ## Build config-extractor dev tool
	@echo "→ Building config-extractor..."
	@mkdir -p bin
	go build -o bin/config-extractor ./cmd/config-extractor
