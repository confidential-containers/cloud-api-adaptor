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

cleanup() {
    rm -f "$attestation_bundle"
}
trap cleanup EXIT SIGINT SIGTERM

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
			digest:          .sourceRepositoryDigest,
			workflowDigest:  .githubWorkflowSHA,
			workflowTrigger: .githubWorkflowTrigger,
			workflowRef:     .githubWorkflowRef,
		}'
)

digest=$(echo "$claims" | jq -r '.digest')
workflow_digest=$(echo "$claims" | jq -r '.workflowDigest')
workflow_trigger=$(echo "$claims" | jq -r '.workflowTrigger')
workflow_ref=$(echo "$claims" | jq -r '.workflowRef')

verification_failed=""

if [ "$digest" != "$expected_digest" ]; then
	echo "Source code digest mismatch: expected $expected_digest, got $digest"
	verification_failed="1"
fi

if [ "$workflow_digest" != "$digest" ]; then
	echo "Workflow digest mismatch: expected $expected_digest, got $workflow_digest"
	verification_failed="1"
fi

if [ "$workflow_trigger" != "push" ] && [ "$workflow_trigger" != "workflow_dispatch" ]; then
	echo "Workflow trigger mismatch: expected push or workflow_dispatch, got $workflow_trigger"
	verification_failed="1"
fi

if [ "$workflow_ref" != "refs/heads/main" ]; then
	echo "Workflow ref mismatch: expected refs/heads/main, got $workflow_ref"
	verification_failed="1"
fi

if [ "$verification_failed" != "" ]; then
	echo "Verification failed"
	exit 1
fi

echo "Verification passed"
