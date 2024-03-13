#!/usr/bin/env bash

set -euo pipefail

check=false
if [[ "${1-}" == "--check" ]]; then
    check=true
fi

# Find top level terraform directories

readarray -t <<< "$(
  find "$(pwd)" -type f -name "*.tf" -exec dirname "{}" \; |
    sort -ud
)"
terraformPaths=("${MAPFILE[@]}")
pathPrefix="${terraformPaths[0]}"
terraformModules=("${pathPrefix}")
for ((i = 1; i < ${#terraformPaths[@]}; i++)); do
  path="${terraformPaths[i]}"
  if [[ ${path} == "${pathPrefix}"* ]]; then
    continue
  fi
  pathPrefix="${path}"
  terraformModules+=("${pathPrefix}")
done

fmtFlags=()
if [ "$check" = true ]; then
  fmtFlags+=("-check")
fi

exitcode=0

echo "Checking Terraform modules:"
for module in "${terraformModules[@]}"; do
  echo "  $module"
  terraform -chdir="${module}" init -backend=false > /dev/null || exitcode=$?
  terraform -chdir="${module}" fmt "${fmtFlags[@]}" -recursive || exitcode=$?
  terraform -chdir="${module}" validate || exitcode=$?
done

exit $exitcode
