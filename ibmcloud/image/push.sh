#!/bin/bash
#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit -o pipefail -o nounset

cd "$(dirname "${BASH_SOURCE[0]}")"

function usage() {
    echo "Usage: $0 --name <image name> --path <image path>"
}

while (( "$#" )); do
    case "$1" in
        --name) image_name=$2 ;;
        --path) image_path=$2 ;;
        --help) usage; exit 0 ;;
        *)      usage 1>&2; exit 1;;
    esac
    shift 2
done

if [[ -z "${image_name-}" || -z "${image_path-}"  ]]; then
    usage 1>&2
    exit 1
fi

export IBMCLOUD_HOME=$(pwd -P)
./login.sh

region=${IBMCLOUD_COS_REGION:-jp-tok}
cos_service_endpoint=${IBMCLOUD_COS_SERVICE_ENDPOINT:-https://s3.jp-tok.cloud-object-storage.appdomain.cloud}
cos_service_instance=$IBMCLOUD_COS_SERVICE_INSTANCE
cos_bucket=$IBMCLOUD_COS_BUCKET

object_key="$(basename "$image_path")"

cos_crn=$(ibmcloud resource service-instance --output json "$cos_service_instance" | jq -r '.[].guid')

ibmcloud cos config auth --method iam
ibmcloud cos config endpoint-url --url "$cos_service_endpoint"
ibmcloud cos config crn --crn "$cos_crn"

echo -e "\nChecking any old image with name \"$image_name\"\n"
timeout=300
interval=15
while image_id=$(ibmcloud is images --output=json --visibility=private | jq -r ".[] | select(.name == \"$image_name\") | .id") && [[ -n "$image_id" ]]; do

    image_json=$(ibmcloud is image --output json "$image_id" || true)
    [[ -z "$image_json" ]] && break

    image_status=$(jq -r .status <<< "$image_json")
    if [[ "$image_status" == available ]]; then

        image_sha256=$(jq -r .file.checksums.sha256 <<< "$image_json")
        if echo "$image_sha256" "$image_path" | sha256sum --check; then
            echo -e "\nImage \"$image_name\" already exists as $image_id with the same content. No need to push\n"
            echo -e "$image_id\n"
            exit 0
        fi

        echo -e "\nImage \"$image_name\" already exists with a different content. Delete the old image\n"
        ibmcloud is image-delete --force "$image_id" || true

        while ibmcloud is image --output json "$image_id" | jq -r '"Image status: \"" + .status + "\""'; do
            sleep $interval
            (( timeout -= $interval ))
        done
    else
        echo "Image \"$image_name\" already exists, and its status is \"$image_status\""
        if (( $timeout <= 0 )); then
            echo "Error: old image $image_id exists, but does not become available in $timeout seconds" 1>&2
            exit 1
        fi
        echo "Check the image status again in $interval seconds..."

        sleep "$interval"
        (( timeout -= $interval ))
    fi
done

echo -e "\nUploading $image_path to cloud object stroage at "$cos_bucket" with key $object_key\n"
./multipart_upload.sh --bucket "$cos_bucket" --file "$image_path"

ibmcloud cos object-head --bucket "$cos_bucket" --key "$object_key"

if [[ ! "$region" =~ - ]]; then 
    location="${region}-geo"
else 
    location=$region
fi
image_ref="cos://$location/$cos_bucket/$object_key"

arch=$(uname -m)
[ "${SE_BOOT:-0}" = "1" ] && os_name="hyper-protect-1-0-s390x" || os_name="ubuntu-20-04-${arch/x86_64/amd64}"

echo -e "\nCreating image \"$image_name\" with $image_ref\n"
image_json=$(ibmcloud is image-create "$image_name" --os-name "$os_name" --file "$image_ref" --output json)
image_id=$(jq -r .id <<< "$image_json")

echo -e "Waiting until image \"$image_name\" becomes available\n"
timeout=300
interval=30
while [[ "$(jq -r .status <<< "$image_json")" != available ]]; do
    jq -r '"Image status: \"" + .status + "\" [" + .status_reasons[].message + "]"' <<< "$image_json"
    if (( $timeout <= 0 )); then
        echo "Error: image $image_id does not become available in $timeout seconds" 1>&2
        exit 1
    fi
    sleep "$interval"
    image_json=$(ibmcloud is image --output json "$image_id")
    (( timeout -= $interval ))
done

echo -e "\nImage \"$image_name\" is now available\n"
echo -e "$image_id\n"
