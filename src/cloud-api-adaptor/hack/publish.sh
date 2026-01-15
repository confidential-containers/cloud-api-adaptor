#!/usr/bin/env bash
#
# Copyright (c) 2024 Intel Corporation
# Copyright (c) 2026 IBM Corporation
#
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit
set -o pipefail
set -o nounset


function _publish_multiarch_manifest()
{
    ${IMAGE_REGISTRY:?}
	${IMAGE_TAGS:?}
    IFS=',' read -ra TAGS <<< "${IMAGE_TAGS}"

    for tag in "${TAGS[@]}"; do
        docker manifest create ${IMAGE_REGISTRY}:${tag} \
            --amend ${IMAGE_REGISTRY}:${tag}-amd64 \
            --amend ${IMAGE_REGISTRY}:${tag}-s390x \
            --amend ${IMAGE_REGISTRY}:${tag}-ppc64le

        docker manifest push ${IMAGE_REGISTRY}:${tag}
    done
}

function main()
{
	action="${1:-}"

	case "${action}" in
		publish-multiarch-manifest) _publish_multiarch_manifest ;;
	esac
}

main "$@"
