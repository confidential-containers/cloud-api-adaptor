#!/bin/bash
# download-image.sh
# takes an image reference and a directory and
# extracts the qcow image into that directory
function usage() {
    echo "Usage: $0 <image> <directory> [-o <name>]"
}

function error() {
    echo "[ERROR] $1" 1>&2 && exit 1
}

image=$1
directory=$2
output=

shift 2
while (( "$#" )); do
    case "$1" in
        -o) output=$2 ;;
        --output) output=$2 ;;
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

# Check if the image name includes "podvm-generic-fedora-s390x-se"
# The "podvm-generic-fedora-s390x-se" docker image is built on s390x host, so here must use s390x platform
if [[ "$image" == *"podvm-generic-fedora-s390x-se"* ]]; then
    platform="s390x"
else
    platform="amd64"
fi

# Create a non-running container to extract image
$container_binary create --platform="$platform" --name "$container_name" "$image" /bin/sh >/dev/null 2>&1;
# Destory container after use
rm-container(){
    $container_binary rm -f "$container_name" >/dev/null 2>&1;
}
trap rm-container 0

podvm=$($container_binary export "$container_name" | tar t | grep podvm)
# Check if file is in image
[ -z "$podvm" ] && error "unable to find podvm qcow2 image"
# If output is not set default to podvm name
[ -z "$output" ] && output="$podvm"

$container_binary cp "$container_name:/$podvm" "$directory/$output"
echo $output
