#!/bin/bash

# Verify Github's attestation reports. Meant to verify binaries built
# by upstream projects (kata-containers and guest-components).
#
# GH cli is used to verify.
#
# Asserts on the claims are:
# - Triggered by push on the main branch
# - Built on the given repository
# - The gh action workflow is matching the given digest
# - The code is matching the given digest
#
# -g will fetch attestation via gh cli, this requires GH_TOKEN to be
# set. By default the attestation will be retrieved by walking the OCI
# manifest

set -euo pipefail

usage() {
	echo "Usage: $0 "
	echo "  -a <oci-artifact w/ sha256 digest>"
	echo "  -d <expected git sha1 from which the artifact was built>"
	echo "  -r <repository on which the artifact was built>"
	echo "  [-g] (optional. fetch attestation using github api)"
	exit 1
}

oci_artifact=""
expected_digest=""
repository=""
github="0"

# Parse options using getopts
while getopts ":a:d:r:g" opt; do
	case "${opt}" in
	a)
		oci_artifact="${OPTARG}"
		;;
	d)
		expected_digest="${OPTARG}"
		;;
	r)
		repository="${OPTARG}"
		;;
	g)
		github="1"
		;;
	*)
		usage
		;;
	esac
done

# Check if all required arguments are provided
if [ -z "${oci_artifact}" ] || [ -z "${expected_digest}" ] || [ -z "${repository}" ]; then
	usage
fi

if [[ "$oci_artifact" =~ @sha256:[a-fA-F0-9]{32}$ ]]; then
	echo "The OCI artifact should be specified using its digest: my-repo.io/my-image@sha256:abc..."
	exit 1
fi

# Convention by gh cli
attestation_bundle="${oci_artifact#*@}.jsonl"

if [ "$github" != "1" ]; then
	attestation_manifest_digest=$(oras discover "$oci_artifact" --format json | jq -r '
		.manifests[]
		| select(.artifactType | test("sigstore.bundle.*json"))
		| .digest
	')

	oci_base="${oci_artifact%@*}"
	attestation_manifest="${oci_base}@${attestation_manifest_digest}"

	attestation_bundle_digest=$(oras manifest fetch "$attestation_manifest" --format json | jq -r '
		.content.layers[]
		| select(.mediaType | test("sigstore.bundle.*json"))
		| .digest
	')

	attestation_image="${oci_base}@${attestation_bundle_digest}"

	oras blob fetch --no-tty "$attestation_image" --output "$attestation_bundle"
else
	gh attestation download "oci://${oci_artifact}" -R "$repository"
fi

claims=$(
	gh attestation verify "oci://${oci_artifact}" \
		-b "$attestation_bundle" \
		-R "$repository" \
		--format json \
		-q '.[].verificationResult.signature.certificate
		| {
			digest:         .sourceRepositoryDigest,
			workflowDigest: .githubWorkflowSHA,
			trigger:        ([.githubWorkflowTrigger, .githubWorkflowRef] | join(":")),
		}'
)

expected_claims="$(jq -n --arg digest "$expected_digest" '{
	digest:         $digest,
	workflowDigest: $digest,
	trigger:        "push:refs/heads/main",
}')"

diff <(jq -S . <<<"$claims") <(jq -S . <<<"$expected_claims") || {
	echo "Verification failed"
	exit 1
}

echo "Verification passed"
