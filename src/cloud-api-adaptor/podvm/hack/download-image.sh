#!/bin/bash
# download-image.sh
# takes an image reference and a directory and
# extracts the container image containing the packer generated qcow pod vm image into that directory
function usage() {
    echo "Usage: $0 <image> <directory> [-o <name>] [-p <platform>] [--pull always(default)|missing|never] [--clean-up]"
}

function error() {
    echo "[ERROR] $1" 1>&2 && exit 1
}

image=$1
directory=$2
output=
platform=
clean_up_image=
pull=always

shift 2
while (( "$#" )); do
    case "$1" in
        -o) output=$2 ;;
        --output) output=$2 ;;
        -p) platform=$2 ;;
        --platform) platform=$2 ;;
        --pull) pull=$2 ;;
        --clean-up) clean_up_image="yes"; shift 1 ;;
        --help) usage; exit 0 ;;
        *)      usage 1>&2; exit 1;;
    esac
    shift 2
done

[[ -z "$image" || -z "$directory" ]] && 1>&2 usage && exit 1

container_name="podvm-exporter-$(date +%Y%m%d%H%M%s)"

# Default to use docker, but check if podman is available
if docker --version >/dev/null 2>&1; then
    container_binary=docker
elif podman --version >/dev/null 2>&1; then
    container_binary=podman
fi

[ -z "$container_binary" ] && error "please install docker or podman"

platform_flag=""
if [[ -n "${platform}" ]]; then
    platform_flag="--platform=${platform}"
elif [[ "${image}" == *"podvm-generic-ubuntu-s390x"* ]]; then
    # The podvm-generic-ubuntu-s390x image (which will be deprecated soon) is built on amd64
    # and therefore has the incorrect platform type, so we need to override it to create the
    # container to extract the image from. Others can use their default platform
    platform_flag="--platform=amd64"
fi

# Create a non-running container to extract image
$container_binary create --pull=${pull} ${platform_flag} --name "$container_name" "$image" /bin/sh >/dev/null 2>&1;
# Destory container after use
rm-container(){
    $container_binary rm -f "$container_name" >/dev/null 2>&1;
    if [ -n "${clean_up_image:-}" ]; then
        $container_binary rmi ${image}
    fi
}
trap rm-container 0

podvm=$($container_binary export "$container_name" | tar t | grep podvm)
# Check if file is in image
[ -z "$podvm" ] && error "unable to find podvm qcow2 image"
# If output is not set default to podvm name
[ -z "$output" ] && output="$podvm"

$container_binary cp "$container_name:/$podvm" "$directory/$output"
echo $output
