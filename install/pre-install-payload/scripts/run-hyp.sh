#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

AWS_EXEC="/opt/confidential-containers/bin/run-aws.sh"
IBMCLOUD_EXEC="/opt/confidential-containers/bin/run-ibmcloud.sh"
LIBVIRT_EXEC="/opt/confidential-containers/bin/run-libvirt.sh"

: "${CAA_PROVIDER:=aws}"

function wait_for_executable() {

    while ! test -f "$1"; do
	  sleep 10
	  echo "Still waiting for $1"
    done
}

function main() { 

    case "$CAA_PROVIDER" in
	aws) 
		wait_for_executable $AWS_EXEC
		$AWS_EXEC 
		;;
	ibmcloud) 
		wait_for_executable $IBMCLOUD_EXEC
		$IBMCLOUD_EXEC 
		;;
	libvirt)
		wait_for_executable $LIBVIRT_EXEC
		$LIBVIRT_EXEC 
		;;
	*)  
		echo "No supported provider found" 
		exit 1
		;;
	     
    esac
}

main 

