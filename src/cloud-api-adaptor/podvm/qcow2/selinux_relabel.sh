#!/bin/bash

FOLDERS="/etc /usr/local/bin /pause_bundle"

for entry in $FOLDERS
do
    sudo restorecon -p -r $entry
done
