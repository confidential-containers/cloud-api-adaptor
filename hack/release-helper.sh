#!/bin/bash

# Check if release tag is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <release_tag>"
    echo 'Please provide a release_tag,eg:
    "v0.8.0-alpha.1" for the confidential containers "0.8.0" release release candidate
    "v0.8.0" for the confidential containers "0.8.0" release'
    exit 1
fi

release_tag=$1
# Check if latest tag retrieval was successful
if [ -z "$release_tag" ]; then
    echo "Please provider release_tag."
    exit 1
fi
# Output the generated tags
echo "The intput release tag: $release_tag"
echo "The follow git commands can be used to do release tags."
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
        echo git push origin "src/$dir/$release_tag"
    fi
done
