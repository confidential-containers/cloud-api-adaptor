#!/usr/bin/env bash

set -euo pipefail

check=false
if [[ "${1-}" == "--check" ]]; then
    check=true

    if ! git diff --quiet --exit-code; then
        echo "Git working tree is dirty, please commit or stash your changes"
        echo "before running ${0} with the --check flag."
        exit 1
    fi
fi

modules=$(find . -name go.mod -exec dirname {} \;)

for module in ${modules}; do
    echo "Tidying Go module $module"
    go mod tidy -C "$module"
done

if [[ "$check" == "true" ]]; then
    git diff --exit-code
fi
