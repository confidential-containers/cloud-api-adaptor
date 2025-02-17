#!/bin/bash
# import.sh takes a podvm docker image reference and a cloud region
# and creates a ibmcloud vpc image of the qcow2 image.
# Requires IBMCLOUD_API_KEY to be set.
# Will default to first bucket it can find, by specifying
# --bucket & --instance to use a particular bucket

error(){
    echo $1 1>&2 && exit 1
}

script_dir=$(cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)

function usage() {
    echo "Usage: $0 <docker-image/qcow2-file> <vpc-region> [--bucket <name> --region <cos-region> --instance <cos-instance> --endpoint <cos-endpoint> --api <cloud-endpoint> --os <operating-system> --pull always(default)|missing|never]"
}

image_file=$1
region=$2
bucket=
bucket_region=$region
instance=
endpoint=
api=${IBMCLOUD_API_ENDPOINT-https://cloud.ibm.com}
platform=
operating_system=
pull=always

shift 2
while (( "$#" )); do
    case "$1" in
        --bucket) bucket=$2 ;;
        --instance) instance=$2 ;;
        --endpoint) endpoint=$2 ;;
        --region) bucket_region=$2 ;;
        --os) operating_system=$2 ;;
        --api) api=$2 ;;
        --platform) platform=$2 ;;
        --pull) pull=$2 ;;
        --help) usage; exit 0 ;;
        *)      usage 1>&2; exit 1;;
    esac
    shift 2
done

if [[ -z "${image_file-}" || -z "${region-}"  ]]; then
    usage 1>&2
    exit 1
fi

[ -z "$IBMCLOUD_API_KEY" ] && error "IBMCLOUD_API_KEY is not set"

IBMCLOUD_API_ENDPOINT="$api" $script_dir/login.sh
ibmcloud target -r "$region"
ibmcloud cos config auth --method iam >/dev/null
ibmcloud cos config region --region "$bucket_region"

current_crn=$(ibmcloud cos config list | grep CRN | tr -s ' ' | cut -d' ' -f2)
instance_crn="$current_crn"
if [ -n "$instance" ]; then
    instance_crn=$(ibmcloud resource service-instance "$instance" --output JSON | jq -r '.[].guid')
fi
if [[ -z "$current_crn" || ! "$current_crn" = "$instance_crn" ]]; then
    ibmcloud cos config crn --crn "$instance_crn" --force
fi

[ -n "$endpoint" ] && ibmcloud cos config endpoint-url --url "$endpoint" >/dev/null

if [ -z "$bucket" ]; then
    for i in $(ibmcloud cos buckets --output JSON | jq -r '.Buckets[].Name'); do
        if ibmcloud cos bucket-head --bucket $i >/dev/null 2>&1; then
            bucket=$i
            break
        fi
    done
    if [ -z "$bucket" ]; then
        error "Can't find any buckets in target region"
    fi
else
    ibmcloud cos bucket-head --bucket "$bucket" || error "Bucket $bucket not found"
fi

if [ -f ${image_file} ]; then
    file=${image_file}
else
    # Download image
    echo "Downloading file from image ${image_file}"
    file=$($script_dir/../../podvm/hack/download-image.sh ${image_file} . --platform "${platform}" --pull "${pull}") || error "Unable to download ${image_file}"
fi

echo "Uploading file ${file}"
# Upload to cos bucket
$script_dir/multipart_upload.sh --file "$file" --bucket "$bucket"

# Clean-up bucket after image creation or error
delete-object() {
    ibmcloud cos object-delete --bucket "$bucket" --key "$file" --force
}
trap delete-object 0

# Create VPC image from cos bucket
location=$bucket_region
[[ ! "$bucket_region" =~ - ]] && location="${bucket_region}-geo"

image_name="${file%.*}"
image_ref="cos://$location/$bucket/$file"
# If OS isn't specified infer from file name
if [ -z "$operating_system" ]; then
    image_arch="${image_name##*-}"
    operating_system="ubuntu-20-04-${image_arch/x86_64/amd64}"
fi
image_name="$(echo ${image_name} | tr '[:upper:]' '[:lower:]' | sed 's/\./-/g' | sed 's/_/-/g')"
image_json=$(ibmcloud is image-create "$image_name" --os-name "$operating_system" --file "$image_ref" --output JSON) || error "Unable to create vpc image $image_name"
image_id=$(echo "$image_json" | jq -r '.id')

echo "Created image $image_name with id $image_id"

timeout=300
interval=10
while [[ "$(ibmcloud is image "$image_id" --output JSON | jq -r .status)" != available ]]; do
    if (( $timeout <= 0 )); then
        error "image $image_id did not become available in $timeout seconds" 1>&2
    fi
    sleep "$interval"
    (( timeout -= $interval ))
done

echo "Image $image_name with id $image_id is available"
