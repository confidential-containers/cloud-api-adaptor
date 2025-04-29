#!/bin/bash

set -euo pipefail

# This script is a shim to allow yq to be used as a drop-in replacement for jq, abstracting away the differences for simple queries between v3 and v4.
# This is the output of yq --version for each:
# yq_v4 --version yq (https://github.com/mikefarah/yq/) version v4.35.1
# yq_v3 --version yq version 3.4.1

usage() { echo "Usage: $0 <query> <path to yaml>"; }

QUERY="${1-}"
YAML_PATH="${2-}"

if [ "$QUERY" == "-h" ]; then
	usage
	exit 0
elif [ -z "${QUERY-}" ] || [ -z "${YAML_PATH-}" ]; then
	usage >&2
	exit 1
fi

# does yq exist in the path?
if ! command -v yq > /dev/null; then
	echo "yq not found in path" >&2
	exit 1
fi

echo "type yq: $(type yq || true)" >&2
echo "command -v yq: $(command -v yq || true)" >&2
echo "which yq: $(which yq || true)" >&2
if yq --version | grep '^.* version v4.*$' > /dev/null; then
	# convert null to empty string
	QUERY="${QUERY} | select(. != null)"
	YQ_VERSION=4
elif yq --version | grep '^.* version 3.*$' > /dev/null; then
	YQ_VERSION=3
	# if the query is prefixed with a dot, remove it
	if [[ "$QUERY" == .?* ]]; then
		QUERY="${QUERY:1}"
	fi
else
	echo "unsupported yq version: $(yq --version || true)" >&2
	exit 1
fi

if [ "$YQ_VERSION" -eq 4 ]; then
	yq "$QUERY" "$YAML_PATH"
else
	yq r "$YAML_PATH" "$QUERY"
fi
