#!/bin/bash
#
# (C) Copyright Confidential Containers Contributors 2023.
# SPDX-License-Identifier: Apache-2.0
#
# Use this script to delete the cloud-api-adaptor images generated from the
# CI workflows.

set -o errexit
set -o nounset
set -o pipefail

ORG=${ORG:-confidential-containers}
PACKAGE=${PACKAGE:-cloud-api-adaptor}
PACKAGE_BASE_API="${PACKAGE_BASE_API:-/orgs}"
REPO=${REPO:-cloud-api-adaptor}
REGISTRY="ghcr.io"

DB_FILE="$(mktemp)"
REFS_DB_FILE="$(mktemp)"

trap '{ rm -f $DB_FILE; rm -f $REFS_DB_FILE; }' EXIT

# Delete dangling images
#
delete_dangling_images() {
	# If the image is not referenced nor tagged then it is dangling.
	while read -r id; do
		if ! grep -q -e "\<$id\>" "$REFS_DB_FILE"; then
			delete_image_by_id "$id"
		fi
	done < <(jq -r '.[].id' "$DB_FILE")
}

# Delete the image by passing its ID.
#
delete_image_by_id() {
	local id="${1}"

	echo "Delete image version=$id"
	gh api --method DELETE "${PACKAGE_BASE_API}/${ORG}/packages/container/${PACKAGE}/versions/${id}"
}

# Delete the tagged image is not enough, we need to remove any manifest images
# first.
#
delete_tagged_image() {
	local image_id="$1"
	local entry

	entry=$(grep -e "\<$image_id\>" "$REFS_DB_FILE")

	# Remove the first two tokens and get the remaining (which are the
	# references)
	for id in $(echo "$entry" | cut -d " " -f3-); do
		delete_image_by_id "$id"
	done

	delete_image_by_id "$image_id"
}

# Loop through CI *tagged* images to delete if the associated pull request
# is closed.
#
delete_images_from_closed_prs() {
	# Each PR has at least one image associated with. Save the PR number on
	# this list to avoid calling the API unnecessarily.
	local closed_prs=""
	local id tag pr image

	while read -r version; do
		id="$(echo "$version" | awk '{print $1}')"
		tag="$(echo "$version" | awk '{print $2}')"
		# Tag format: ci-prN and ci-prN-dev, where N is the PR number
		pr="$(echo "$tag" | sed -e 's/ci-pr//' -e 's/-dev//')"

		if ! echo "$closed_prs" | grep -q "\<$pr\>"; then
			state=$(gh api -q '.state' "/repos/${ORG}/${REPO}/pulls/${pr}")
			[ "$state" = "closed" ] || continue
			closed_prs+=" $pr"
		fi

		echo "PR ${REPO}/pull/${pr} is closed"
		image="${REGISTRY}/${ORG}/${PACKAGE}:${tag}"
		delete_tagged_image "${id}" || \
		{ echo "Failed to delete image ${image}"; continue; }
		echo "Deleted image ${image}"
	done < <(get_tagged_images)
}

# Generate two databases:
#
# 1. A simple bump of the Github's packages JSON
# 2. A plain-text file mapping tagged image to manifest references. Each line with format:
#    TAGGED_IMAGE_ID IMAGE_TAG REF_1_ID REF_2_ID ...
#
generate_dbs() {
	local image refs sha1

	# TODO: the API is limited to return 100 entries maximum. We may need
	# implement a paginator.
	gh api "${PACKAGE_BASE_API}/${ORG}/packages/container/${PACKAGE}/versions?per_page=100" > "$DB_FILE"

	while read -r id tag; do
		refs=""
		image="${REGISTRY}/${ORG}/${PACKAGE}:${tag}"
		while read -r digest; do
			sha1=$(jq -r '.[] | select(.name == "'"${digest}"'") | .id' "$DB_FILE")
			refs+="$sha1 "
		done < <(docker manifest inspect "$image" | jq -r '.manifests[].digest')
		echo "$id $tag $refs" >> "$REFS_DB_FILE"
	done < <(get_tagged_images)
}

# Get tagged images
# Returns "ID TAG" lines
#
get_tagged_images() {
	while read -r version; do
		id="$(echo "$version" | awk '{print $1}')"
		tag="$(echo "$version" | awk '{print $2}')"
		echo "$id $tag"
	done < <(jq -r '.[] | select(.metadata.container.tags[0] != null) | "\(.id) \(.metadata.container.tags[0])"' "$DB_FILE")
}

main() {
	for cmd in gh jq docker; do
		command -v "$cmd" >/dev/null || {
			echo "Unabled to find the '$cmd' command";
			exit 1;
		}
	done

	[ -n "${GITHUB_TOKEN:-}" ] || {
		echo "GITHUB_TOKEN should be exported"
		exit 1
	}

	# Generate the databases
	generate_dbs

	echo "::group::Delete images from closed PRs"
	delete_images_from_closed_prs
	echo "::endgroup::"

	echo "::group::Delete dangling images"
	delete_dangling_images
	echo "::endgroup::"
}

main "$@"