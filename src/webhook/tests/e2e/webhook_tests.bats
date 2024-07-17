#!/usr/bin/env bats
#
# Copyright Confidential Containers Contributors
#
# SPDX-License-Identifier: Apache-2.0
#
# End-to-end tests.
#
test_tags="[webhook]"

# Assert that the pod mutated as expected.
#
# Parameters:
# 	$1: the expected VM limits
# 	$2: the expected VM requests
#
# Global variables:
# 	$pod_file: path to the pod configuration file.
#
assert_pod_mutated() {
	local expect_vm_limits="$1"
	local expect_vm_requests="$2"

        local actual_vm_limits=$(kubectl get -f "$pod_file" \
                -o jsonpath='{.spec.containers[0].resources.limits.kata\.peerpods\.io/vm}')
        echo "VM limits expected: $expect_vm_limits, actual: $actual_vm_limits"
        [ $expect_vm_limits -eq $actual_vm_limits ]

        local actual_vm_requests=$(kubectl get -f "$pod_file" \
                -o jsonpath='{.spec.containers[0].resources.requests.kata\.peerpods\.io/vm}')
        echo "VM requests expected: $expect_vm_requests, actual: $actual_vm_requests"
        [ $expect_vm_requests -eq $actual_vm_requests ]
}

setup_file() {
	export project_dir="$(cd ${BATS_TEST_DIRNAME}/../.. && pwd)"
	echo "Create runtimeClass"
	kubectl apply -f "$project_dir/hack/rc.yaml"
}

setup() {
	export pod_file="$project_dir/hack/pod.yaml"
}

teardown() {
	kubectl delete -f "$pod_file" || true
}

@test "$test_tags test it can mutate a pod" {
	kubectl apply -f "$pod_file"
	assert_pod_mutated 1 1
}

@test "$test_tags test it should not mutate non-peerpods" {
	echo "Create a pod without runtimeClassName"
	cat "$pod_file" | sed -e 's/^\s*runtimeClassName:.*//' | \
		kubectl apply -f -

	! kubectl get -f ../../hack/pod.yaml -o json | \
		grep kata\.peerpods
}

@test "$test_tags test default parameters can be changed" {
	skip "TODO: This test is not passing"
	local runtimeclass="kata-wh-test"

	# Create a dummy runtimeClass to use on this test.
	cat <<-EOF | kubectl apply -f -
	apiVersion: node.k8s.io/v1
	handler: ${runtimeclass}
	kind: RuntimeClass
	metadata:
	  name: ${runtimeclass}
	overhead:
	  podFixed:
	    memory: "120Mi"
	    cpu: "250m"
	EOF

	kubectl set env deployment/peer-pods-webhook-controller-manager \
		-n peer-pods-webhook-system TARGET_RUNTIMECLASS="$runtimeclass"

	cat "$pod_file" | sed -e 's/^\(\s*runtimeClassName:\).*/\1 '${runtimeclass}'/' | \
		kubectl apply -f -

	kubectl get -f $pod_file -o json
	assert_pod_mutated 1 1
}
