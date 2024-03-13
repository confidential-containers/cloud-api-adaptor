# (C) Copyright Confidential Containers Contributors
# SPDX-License-Identifier: Apache-2.0

#include Makefile.defaults

.PHONY: check fmt

SHELL = bash -o pipefail

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

 ## Run formatters and linters against the code.
.PHONY: check
check: fmt golangci-lint shellcheck tidy-check govulncheck packer-check terraform-check

.PHONY: fmt
fmt: ## Run go fmt against code.
	find . -name '*.go' -print0 | xargs -0 gofmt -l -s -w

.PHONY: shellcheck
shellcheck: ## Run shellcheck against shell scripts.
	./hack/shellcheck.sh

.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint against code.
	./hack/golangci-lint.sh -v

.PHONY: tidy
tidy:
	./hack/go-tidy.sh

.PHONY: tidy-check
tidy-check:
	./hack/go-tidy.sh --check

.PHONY: govulncheck
govulncheck:
	./hack/govulncheck.sh

.PHONY: packer-format
packer-format:
	./hack/packer-check.sh

.PHONY: packer-check
packer-check:
	./hack/packer-check.sh --check

.PHONY: terraform-format
terraform-format:
	./hack/terraform-check.sh

.PHONY: terraform-check
terraform-check:
	./hack/terraform-check.sh --check
