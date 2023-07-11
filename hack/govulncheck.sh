#!/usr/bin/env bash

set -euo pipefail

function echoerr() {
    echo "$@" 1>&2
}

function usage() {
    echoerr govulncheck wrapper script
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

# Check if govulncheck is installed
if ! command -v govulncheck &> /dev/null; then
    echoerr "govulncheck is not installed, please install it by running:"
    echoerr "go install golang.org/x/vuln/cmd/govulncheck@latest"
    exit 1
fi

readarray -t <<< "$(find . -name go.mod -exec sh -c 'dirname $1' shell {} \;)"
goModules=("${MAPFILE[@]}")

if [ "$verbose" = true ]; then
    echo "Found the following Go modules:"
    for module in "${goModules[@]}"; do
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
fi

statuscode=0

for module in "${goModules[@]}"; do
        govulncheck -C "${module}" ./... || statuscode=$?
done

exit $statuscode
