#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")

tags_file="${script_dir}/../tags.txt"
registry="${registry:-quay.io/confidential-containers}"
name="cloud-api-adaptor"
arch_file_prefix="tags-architectures-"
all_arches_array=()
dev_tags_array=()
release_tags_array=()

function get_all_arches() {
    all_arches_array=()
    local arch=""
    pushd "${script_dir}/.."
    i=0
    #shellcheck disable=SC2010
    while read line
    do
        arch=${line#"$arch_file_prefix"}
        all_arches_array[$i]=$arch
        i=$((i+1))
    done < <(ls -1 |grep $arch_file_prefix)
    popd
    print_all_arches
}

function get_dev_tags() {
    dev_tags_array=()
    local dev_tags=""
    pushd "${script_dir}/.."
    if [[ -f $tags_file ]]; then
        dev_tags_str=$(cat $tags_file | grep dev_tags)
        dev_tags=${dev_tags_str#"dev_tags="}
    else
        echo "Did not find file: $tags_file"
        popd
        exit 99
    fi
    IFS=',' read -ra dev_tags_array <<< "$dev_tags"
    for i in "${dev_tags_array[@]}"; do
        echo $i
    done
    popd
    print_dev_tags
}

function get_release_tags() {
    release_tags_array=()
    local release_tags=""
    pushd "${script_dir}/.."
    if [[ -f $tags_file ]]; then
        release_tags_str=$(cat $tags_file | grep release_tags)
        release_tags=${release_tags_str#"release_tags="}
    else
        echo "Did not find file: $tags_file"
        popd
        exit 99
    fi
    IFS=',' read -ra release_tags_array <<< "$release_tags"
    for i in "${release_tags_array[@]}"; do
      echo $i
    done
	popd
    print_release_tags
}

function print_release_tags() {
    for tag in "${release_tags_array[@]}"; do
        echo $tag
    done
}

function print_dev_tags() {
    for tag in "${dev_tags_array[@]}"; do
        echo $tag
    done
}

function print_all_arches() {
    for arch in "${all_arches_array[@]}"; do
        echo $arch
    done
}

function pull_an_arch_tag_image() {
    local tag=""
    local arch=""
    tag=$1
    arch=$2

    echo "Pulling image: ${registry}/${name}:$tag-$arch"
    docker pull --platform="linux/$arch" "${registry}/${name}:$tag-$arch"
    docker inspect "${registry}/${name}:$tag-$arch" |grep Architecture
}

function pull_all_images() {
    for tag in "${dev_tags_array[@]}"; do
        for arch in "${all_arches_array[@]}"; do
            pull_an_arch_tag_image "$tag" "$arch"
        done
    done

    for tag in "${release_tags_array[@]}"; do
        for arch in "${all_arches_array[@]}"; do
            pull_an_arch_tag_image "$tag" "$arch"
        done
    done
}

function generate_push_a_manifest() {
    local tag=""
    tag=$1

    manifest="${registry}/${name}:$tag"
    arch_images=()

    i=0
    for arch in "${all_arches_array[@]}"; do
        arch_images[$i]="$manifest-$arch"
        i=$((i+1))
    done

    echo "Manifest: ${manifest}"
    docker buildx imagetools create -t "${manifest}" "${arch_images[@]}"
}

function generate_push_all_manifests() {
    for tag in "${dev_tags_array[@]}"; do
        generate_push_a_manifest "$tag"
    done

    for tag in "${release_tags_array[@]}"; do
        generate_push_a_manifest "$tag"
    done
}

function verify_a_manifest() {
    local tag=""
    tag=$1

    manifest="${registry}/${name}:$tag"
    for arch in "${all_arches_array[@]}"; do
        docker pull --platform="linux/$arch" "${registry}/${name}:$tag"
        docker inspect "${registry}/${name}:$tag" |grep Architecture
    done
    
}

function verify_all_manifests() {
    for tag in "${dev_tags_array[@]}"; do
        verify_a_manifest "$tag"
    done

    for tag in "${release_tags_array[@]}"; do
        verify_a_manifest "$tag"
    done
}

# main
get_all_arches
get_dev_tags
get_release_tags
pull_all_images
generate_push_all_manifests
verify_all_manifests
