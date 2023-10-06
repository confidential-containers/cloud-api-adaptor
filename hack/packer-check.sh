#!/usr/bin/env bash

set -euo pipefail

check=false
if [[ "${1-}" == "--check" ]]; then
    check=true
fi

# Find top level packer directories

readarray -t <<< "$(
  find "$(pwd)" -type f -name "*.pkr.hcl" -exec dirname "{}" \; |
    sort -ud
)"
packerPaths=("${MAPFILE[@]}")
pathPrefix="${packerPaths[0]}"
packerModules=("${pathPrefix}")
for ((i = 1; i < ${#packerPaths[@]}; i++)); do
  path="${packerPaths[i]}"
  if [[ ${path} == "${pathPrefix}"* ]]; then
    continue
  fi
  packerModules+=("${pathPrefix}")
  pathPrefix="${path}"
done

fmtFlags=()
if [ "$check" = true ]; then
  fmtFlags+=("-check" "-write=true" "-diff")
fi

exitcode=0

echo "Checking Packer modules:"
for module in "${packerModules[@]}"; do
  echo "  $module"
  # packer init "${module}"
  packer fmt "${fmtFlags[@]}" "${module}" || exitcode=$?
  # packer validate "${module}" || exitcode=$?
done

exit $exitcode
