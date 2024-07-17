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

# Wait till deployment pods are ready and the rollout is successful
# Parameters
#	$1: deployment name
#	$2: namespace
wait_for_deployment() {
	local deployment=$1
	local namespace=$2
	local timeout=300
	local interval=5
	local elapsed=0
	local ready=0

	while [ $elapsed -lt $timeout ]; do
		# Check the rollout status of the deployment
		rollout_status=$(kubectl rollout status deployment "$deployment" -n "$namespace")
		if [[ $rollout_status == *"successfully rolled out"* ]]; then
			echo "$deployment has been successfully rolled out"

			# Check if all the replicas are ready
			ready=$(kubectl get deployment -n "$namespace" "$deployment" -o jsonpath='{.status.readyReplicas}')
			replicas=$(kubectl get deployment -n "$namespace" "$deployment" -o jsonpath='{.status.replicas}')
			if [ "$ready" == "$replicas" ]; then
				echo "$deployment is ready"
				return 0
			fi
		fi
		sleep $interval
		elapsed=$((elapsed + interval))
	done
	echo "$deployment is not ready after $timeout seconds"
	return 1
}

# Check env var value for the pods in a deployment
# Parameters
#	$1: deployment name
#	$2: namespace
#	$3: env var key
#	$4: expected value
check_deployment_pods_env_var() {
	local deployment=$1
	local namespace=$2
	local env_key=$3
	local expected_value=$4

	# Get all pods in the deployment
	pods=$(kubectl get pods -n $namespace -l app=$deployment -o jsonpath='{.items[*].metadata.name}')

	for pod in $pods; do
		actual_value=$(kubectl get pod $pod -n $namespace -o jsonpath='{.spec.containers[0].env[?(@.name=="$env_key")].value}')
		if [ "$actual_value" != "$expected_value" ]; then
			echo "Pod $pod does not have the expected $env_key value. Expected: $expected_value, Found: $actual_value"
			return 1
		fi
	done

	echo "All pods have the correct $env_key value: $expected_value"
	return 0
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

	# Wait for the controller pods to be ready.
	wait_for_deployment peer-pods-webhook-controller-manager peer-pods-webhook-system

	# Check the env var TARGET_RUNTIMECLASS value for the pods in the deployment
	check_deployment_pods_env_var peer-pods-webhook-controller-manager peer-pods-webhook-system TARGET_RUNTIMECLASS $runtimeclass

	cat "$pod_file" | sed -e 's/^\(\s*runtimeClassName:\).*/\1 '${runtimeclass}'/' | \
		kubectl apply -f -

	assert_pod_mutated 1 1
}
