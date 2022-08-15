#!/bin/bash
#
# Copyright Confidential Containers Contributors
#

set -o errexit -o pipefail -o nounset

function usage() {
    echo "Usage: $0 --file <file name> --bucket <bucket name> [--size <size of parts 20M-100M>]"
}

# cleanup, removes the split files and the structure if they have been created by this script
function cleanup() {
    rm -f "${original_file}_structure.json"
    rm -f "$original_file".*
}
trap cleanup EXIT

while (( "$#" )); do
    case "$1" in
        --file) original_file=$2 ;;
        --bucket) bucket=$2 ;;
        --size) split_size=$2 ;;
        --help) usage; exit 0 ;;
        *)      usage 1>&2; exit 1;;
    esac
    shift 2
done

# Default to 100M part size
split_size=${split_size:-100M}

# Requires file and bucket to be specified to continue
if [[ -z "${original_file-}" || -z "${bucket-}"  ]]; then
    usage 1>&2
    exit 1
fi

# Divide the original file up into smaller sections
split "$original_file" -b $split_size "$original_file."
# Create a multipart-upload, need the upload-id for part uploads 
upload_id=$(ibmcloud cos multipart-upload-create --bucket "$bucket" --key "$original_file" --output JSON | jq -r '.UploadId')
if [[ -z "$upload_id" ]]; then 
    echo "Unable to start upload, check permissions" 1>&2
    exit 1
else
    echo "Uploading to "$upload_id""
fi

# The complete command requires a description of the parts to piece together into the final object
# This is constructed in JSON, the end result looks something like
# {"Parts": [ {"ETag": "tagabcdef", "PartNumber": 1 } ... ]}
structure="{\"Parts\":[]}"
i=1
for part in "$original_file".*;
do 
    echo "Uploading part ${part} to bucket ${bucket}";
    etag=$(ibmcloud cos part-upload --bucket "$bucket" --key "$original_file" --upload-id "$upload_id" --part-number $i --body "$part" --output JSON | jq -r '.ETag')
    structure=$(echo "$structure" | jq --arg etag "$etag" --argjson i $i '.Parts += [{"ETag": $etag, "PartNumber": $i}]')
    ((i=i+1))
done

# Temporarily store the structure to file for the cli command
echo "$structure" > "${original_file}_structure.json"
ibmcloud cos multipart-upload-complete --bucket "$bucket" --key "$original_file" --upload-id "$upload_id" --multipart-upload "file://${original_file}_structure.json"
