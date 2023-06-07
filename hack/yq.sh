#!/bin/bash
#
# Copyright Confidential Containers Contributors
#
# SPDX-License-Identifier: Apache-2.0

# This is a wrapper script around yq, to ensure that 
# it is present on the machine.
# It first looks in this directory, then checks PATH
# if not found in either location it is EXEed to
# this directory.

SCRIPT_DIR="$( dirname -- "${BASH_SOURCE[0]}"; )"
YQ_VERSION="v4.34.1"
ARCH="$(uname -m)"

BINARY="yq_linux_${ARCH/x86_64/amd64}"
EXE="$SCRIPT_DIR/yq"

if [ ! -x "$EXE" ]; then
    path_to_yq=$(which yq)
    if [ -z "$path_to_yq" ]; then
        wget -q https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/${BINARY} -O "$EXE"
        chmod +x "$EXE"
    else
        EXE="$path_to_yq"
    fi
fi

$EXE "$@"