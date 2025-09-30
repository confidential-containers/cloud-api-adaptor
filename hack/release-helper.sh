#!/bin/bash

script_name="$(basename "${BASH_SOURCE[0]}")"

function update_tags() {
    # Check if release tag is provided
    if [ $# -eq 0 ]; then
        echo "Usage: $script_name go-tag <release_tag> [<remote_name>]"
        echo 'Please provide a release_tag eg:
        "v0.8.0-alpha.1" for the confidential containers "0.8.0" release candidate
        "v0.8.0" for the confidential containers "0.8.0" release'
        exit 1
    fi

    release_tag=$1
    # Check if latest tag retrieval was successful
    if [ -z "$release_tag" ]; then
        echo "Please provide release_tag."
        exit 1
    fi

    remote_name="${2:-origin}"

    # Output the generated tags
    echo "The input release tag: $release_tag"
    echo "The follow git commands can be used to do create (pre-)release tags."
    echo "*****************************IMPORTANT********************************************
    After a tag has been set, it cannot be moved!
    The Go module proxy caches the hash of the first tag and will refuse any update.
    If you mess up, you need to restart the tagging with the next patch version.
    **********************************************************************************"
    # Change to the root directory of your project
    cd src || exit

    # Iterate over the directories
    for dir in *; do
        # Check if the item is a directory
        if [ -d "$dir" ]; then
            # Tag the current state
            echo git tag "src/$dir/$release_tag" main

            # Push the tag to the remote repository
            echo git push "${remote_name}" "src/$dir/$release_tag"
        fi
    done
}

update_provider_overlays() {

    # Check if release tag is provided
    if [ $# -eq 0 ]; then
        echo "Usage: $script_name caa-image-tag <image_tag>"
        echo 'Please provide a image_tag of the pre-release tested CAA image from
        quay.io/confidential-containers/cloud-api-adaptor'
        exit 1
    fi

    image_tag=$1

    pushd src/cloud-api-adaptor/install/overlays/ || exit
    for provider in *; do
        if [ "${provider}" == "alibabacloud" ] ; then
            # The alibabacloud image is managed in a separate mirror
            continue
        fi

        pushd "${provider}" || exit

        # libvirt uses the dev built image
        tag_prefix=""
        if [ "${provider}" == "libvirt" ] || [ "${provider}" == "docker" ] ; then
            tag_prefix="dev-"
        fi

        # yq and kustomize edit both reformat the file, so fall back to using sed :(
        sed_inplace=(-i)
        # BSD Sed compatbility
        if ! sed --version >/dev/null 2>&1; then
            sed_inplace=(-i "")
        fi
        sed "${sed_inplace[@]}" "s/^\(.*newTag:\).*/\1 ${tag_prefix}${image_tag}/g" kustomization.yaml
        popd || exit
    done
    popd || return
}


usage() {
    cat <<-EOF
    Utility to help with release process.

    Use: $script_name [-h|--help] <command> <parameters>, where:
    -h | --help : show this usage
    command : Select the function to use. Can be:
        "go-tag": Generates the release tags for the go modules in the project.
            - Parameters: <release_tag> [<remote_name>] where:
                - release_tag is the version of the release
                - remote_name is the optional name of the remote, upstream branch
                (defaults to origin)
        "caa-image-tag": Updates the install overlay kustomization files to specific
        image tag of the cloud-api-adaptor to use for the release, to provide a
        pinned and stable version
            - Parameters: <image_tag> where
                - image_tag corresponds to the tag of the pre-release tested version
                of the quay.io/confidential-containers/cloud-api-adaptor image
EOF
}

main() {
    command=$1
    shift #strip command when passing to functions
    case $command in
        ''|-h|--help)
            usage && exit 0;;
        go-tag)
            update_tags "$@"
            ;;
        caa-image-tag)
            update_provider_overlays "$@"
            ;;
        *)
            echo "::error:: Unknown command '$command'"
            usage && exit 1
    esac
}

main "$@"
