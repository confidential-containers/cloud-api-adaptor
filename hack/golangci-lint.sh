#!/usr/bin/env bash
#
# Copyright Confidential Containers Contributors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

function echoerr() {
    echo "$@" 1>&2
}

function usage() {
    echoerr golangci-lint wrapper script
    echoerr "Usage: $0 [-v]"
    echoerr
    echoerr "Options:"
    echoerr "  -v     verbose output"
}

# Parse flags
verbose=false
while getopts "v" opt; do
    case $opt in
        v)
            verbose=true
            ;;
        \?)
            echoerr # newline
            usage
            exit 1
            ;;
    esac
done

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echoerr "Go is not installed, please install it."
    echoerr "You can find installation instructions at https://golang.org/doc/install."
    exit 1
fi

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    echoerr "golangci-lint is not installed, please install it."
    echoerr "You can find installation instructions at https://golangci-lint.run/usage/install/."
    exit 1
fi

# Configuration
excludeModules=(
    "./src/cloud-api-adaptor/podvm" # see the comment in podvm/go.mod
)
flags=()

readarray -t <<< "$(find . -name go.mod -exec sh -c 'dirname $1' shell {} \;)"
goModules=("${MAPFILE[@]}")

if [ "$verbose" = true ]; then
    echo "Found the following Go modules:"
    for module in "${goModules[@]}"; do
        echo "  $module"
    done
    echo # newline
fi

# Exclude modules
excluded=()
for i in "${!goModules[@]}"; do
    for exclude in "${excludeModules[@]}"; do
        if [[ "${goModules[i]}" == "$exclude" ]]; then
            excluded+=("${goModules[i]}")
            unset 'goModules[i]'
        fi
    done
done
if [ "$verbose" = true ]; then
    echo "Excluded the following Go modules based on exclude list:"
    for module in "${excluded[@]}"; do
        echo "  $module"
    done
    echo # newline
fi

# Exclude gitignored modules
gitignored=()
for i in "${!goModules[@]}"; do
    if git check-ignore -q "${goModules[$i]}"; then
        gitignored+=("${goModules[i]}")
        unset 'goModules[i]'
    fi
done
if [ "$verbose" = true ]; then
    echo "Excluded the following Go modules because they are gitignored:"
    for module in "${gitignored[@]}"; do
        echo "  $module"
    done
    echo # newline
fi

if [ "$verbose" = true ]; then
    echo "Checking ${#goModules[@]} Go modules:"
    for module in "${goModules[@]}"; do
        echo "  $module"
    done
    echo # newline

    flags+=("--verbose")
fi

statuscode=0

for module in "${goModules[@]}"; do
    pushd "$module" >/dev/null
    golangci-lint run "${flags[@]}" || statuscode=$?
    popd >/dev/null
done

exit $statuscode
