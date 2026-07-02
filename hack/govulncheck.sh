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
    echoerr
    echoerr "If .govulncheck-ignore.toml exists at the repo root, vulnerability IDs listed"
    echoerr "there are suppressed and do not cause a non-zero exit. Requires python3."
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

ignore_file=""
# Use an absolute path so it resolves correctly regardless of working directory
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
if [[ -f "${repo_root}/.govulncheck-ignore.toml" ]]; then
    ignore_file="${repo_root}/.govulncheck-ignore.toml"
    if [ "$verbose" = true ]; then
        echo "Using ignore file: ${ignore_file}"
    fi
    if ! command -v python3 &> /dev/null; then
        echoerr "python3 is required when .govulncheck-ignore.toml is present but was not found."
        exit 1
    fi
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
    # Check if the module has any Go packages before running govulncheck
    # Some modules have go.mod but no Go source files
    if go list -C "${module}" ./... 2>&1 | grep -q "matched no packages"; then
        if [ "$verbose" = true ]; then
            echo "Skipping ${module}: no Go packages found"
        fi
        continue
    fi
    
    if [[ -n "${ignore_file}" ]]; then
        if ! govulncheck --json -C "${module}" ./... \
            | IGNORE_FILE="${ignore_file}" VERBOSE="${verbose}" \
            python3 "$(dirname "$0")/govulncheck-filter.py"; then
            statuscode=1
            echoerr
            echoerr "Re-running govulncheck for human-readable output (non-ignored findings in ${module}):"
            echoerr
            govulncheck -C "${module}" ./... 1>&2 || true
        fi
    else
        govulncheck -C "${module}" ./... || statuscode=$?
    fi
done

exit $statuscode
