#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

CONFIGMAP_NAME=hyp-env-cm
NAMESPACE=confidential-containers-system

die() {
	msg="$*"
	echo "ERROR: $msg" >&2
	exit 1
}

function install_artifacts() {
	echo "Copying cloud-api-adaptor artifacts onto host"

	local artifacts_dir="/opt/confidential-containers-pre-install-artifacts"

	cp -a ${artifacts_dir}/etc/systemd/system/* /etc/systemd/system/
	cp -a ${artifacts_dir}/scripts /opt/confidential-containers/
}

function uninstall_artifacts() {
	echo "Removing cloud-api-adaptor artifacts from host"

	rm -f /etc/systemd/system/remote-hyp.service
	rm -fr /opt/confidential-containers/scripts
}


function copy_provider_config() {
	echo "Copying cloud provider config to /run/hyp.env"

	kubectl get configmap $CONFIGMAP_NAME -n $NAMESPACE -o "jsonpath={ .data['hyp\.env']}" > /run/hyp.env
}

function remove_provider_config() {
	echo "Removing cloud provider config"
	cp /dev/null /run/hyp.env
}

label_node() {
	case "${1}" in
	install)
		kubectl label node "${NODE_NAME}" cc-preinstall/done=true
		;;
	uninstall)
		kubectl label node "${NODE_NAME}" cc-postuninstall/done=true
		;;
	*)
		;;
	esac
}

function print_help() {
	echo "Help: ${0} [install/uninstall]"
}

function main() {
	# script requires that user is root
	local euid=$(id -u)
	if [ ${euid} -ne 0 ]; then
		die "This script must be run as root"
	fi

	local action=${1:-}
	if [ -z "${action}" ]; then
		print_help && die ""
	fi


	case "${action}" in
	install)
		install_artifacts
		copy_provider_config
	        systemctl daemon-reload
	        systemctl start remote-hyp.service
		;;
	uninstall)
	        systemctl stop remote-hyp.service
		remove_provider_config
		uninstall_artifacts
	        systemctl daemon-reload
		;;
	*)
		print_help
		;;
	esac

	label_node "${action}"


	# It is assumed this script will be called as a daemonset. As a result, do
	# not return, otherwise the daemon will restart and reexecute the script.
	sleep infinity
}

main "$@"
