#!/usr/bin/env bash
#
# Copyright (c) 2023 Intel Corporation
# Copyright (c) 2025 IBM Corporation
#
# SPDX-License-Identifier: Apache-2.0
#

set -o errexit
set -o nounset
set -o pipefail

TARGET_BRANCH=${TARGET_BRANCH:-main}

function add_git_config_info() {
	echo "Adding user name and email to the local git repo"

	git config user.email "dummy@email.com"
	git config user.name "Dummy User Name"
}

function rebase_atop_of_the_latest_target_branch() {
	if [ -n "${TARGET_BRANCH}" ]; then
		echo "Rebasing atop of the latest ${TARGET_BRANCH}"
		# Recover from any previous rebase left halfway
		git rebase --abort 2> /dev/null || true
		if ! git rebase "origin/${TARGET_BRANCH}"; then
			# if GITHUB_WORKSPACE is defined and an architecture is not equal to x86_64
			# (mostly self-hosted runners), then remove the repository
			if [ -n "${GITHUB_WORKSPACE:-}" ] && [ "$(uname -m)" != "x86_64" ]; then
				echo "Rebase failed, cleaning up a repository for self-hosted runners and exiting"
				cd "${GITHUB_WORKSPACE}"/..
				sudo rm -rf "${GITHUB_WORKSPACE}"
			else
				echo "Rebase failed, exiting"
			fi
			exit 1
		fi
	fi
}

# Remove unnecessary directories on the github runner to relieve disk space issues
function clean_up_runner() {
	rm -rf /usr/local/.ghcup
	rm -rf /opt/hostedtoolcache/CodeQL
	rm -rf /usr/local/lib/android
	rm -rf /usr/share/dotnet
	rm -rf /opt/ghc
	rm -rf /usr/local/share/boost
	rm -rf "$AGENT_TOOLSDIRECTORY"
	rm -rf /usr/lib/jvm
	rm -rf /usr/share/swift
	rm -rf /usr/local/share/powershell
	rm -rf /usr/local/julia*
	rm -rf /opt/az
	rm -rf /usr/local/share/chromium
	rm -rf /opt/microsoft
	rm -rf /opt/google
	rm -rf /usr/lib/firefox
}


function main() {
	action="${1:-}"

	 add_git_config_info

	case "${action}" in
		clean-up-runner) clean_up_runner;;
		rebase-atop-of-the-latest-target-branch) rebase_atop_of_the_latest_target_branch;;
		*) >&2 echo "Invalid argument"; exit 2 ;;
	esac
}

main "$@"
