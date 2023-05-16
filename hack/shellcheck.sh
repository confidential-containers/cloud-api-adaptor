#!/usr/bin/env bash
#
# Copyright Confidential Containers Contributors
#
# SPDX-License-Identifier: Apache-2.0

function echoerr() {
    echo "$@" 1>&2
}

function usage() {
    echoerr ShellCheck wrapper script
    echoerr "Usage: $0 [-v]"
    echoerr
    echoerr "Options:"
    echoerr "  -v     verbose output"
}

# Check if shellcheck is installed
if ! command -v shellcheck &> /dev/null; then
    echoerr "shellcheck is not installed, please install it."
    echoerr "You can find installation instructions at https://github.com/koalaman/shellcheck#installing."
    exit 1
fi

# Configuration
scanDir="." # Directory to scan
shellcheckOpts=( # Options to pass to ShellCheck
    "--severity=warning"
)
ignorePaths=( # Paths to ignore when running ShellCheck
    "*./.git/*"
    "*.go"
)
includeFiles=( # Additional files to include when running ShellCheck
)

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

set -f # temporarily disable globbing

# Parse excluded files
declare -a excludes
for path in "${ignorePaths[@]}"; do
    [[ -z "$path" ]] && continue
    excludes+=("-not" "-path" "${path}")
done

# Parse additional files
declare -a includes
for file in "${includeFiles[@]}"; do
    [[ -z "$path" ]] && continue
    includes+=("-o" "-name" "${file}")
done

# Find files
declare -a filepaths
shebangregex="^#! */[^ ]*/(env *)?[abk]*sh"

while IFS= read -r -d '' file; do
    filepaths+=("$file")
done < <(find "${scanDir}" \
    "${excludes[@]}" \
    -type f \
    '(' \
    -name '*.bash' \
    -o -name '*.ksh' \
    -o -name '*.zsh' \
    -o -name '*.sh' \
    -o -name '*.shlib' \
    "${includes[@]}" \
    ')' \
    -print0)

while IFS= read -r -d '' file; do
    head -n1 "$file" | grep -Eqs "$shebangregex" || continue
    filepaths+=("$file")
done < <(find "${scanDir}" \
    "${excludes[@]}" \
    -type f \
    ! -name '*.*' \
    -perm /111 \
    -print0)

# Exclude gitignored modules
gitignored=()
for i in "${!filepaths[@]}"; do
    if git check-ignore -q "${filepaths[$i]}"; then
        gitignored+=("${filepaths[i]}")
        unset 'filepaths[i]'
    fi
done
if [ "$verbose" = true ]; then
    echo "Excluded the following scripts because they are gitignored:"
    for module in "${gitignored[@]}"; do
        echo "  $module"
    done
    echo # newline
fi

# Print files
if [[ "${verbose}" == "true" ]]; then
    echo "Checking ${#filepaths[@]} files:"
    for file in "${filepaths[@]}"; do
        echo "  $file"
    done
fi

statuscode=0

shellcheck \
    "${shellcheckOpts[@]}" \
    "${filepaths[@]}" || statuscode=$?

set +f # re-enable globbing

exit ${statuscode}
