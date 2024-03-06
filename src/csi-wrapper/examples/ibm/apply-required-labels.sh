#!/bin/bash

function help()
{
	echo "You need to run this script like as follows ...."
	echo "./apply-required-setup.sh <node-name> <instanceID> <region-of-instanceID> <zone-of-instanceID>"
	exit 1
}

function apply_labels()
{
	kubectl label nodes $1 "ibm-cloud.kubernetes.io/worker-id"=$2
	kubectl label nodes $1 "failure-domain.beta.kubernetes.io/region"=$3
	kubectl label nodes $1 "failure-domain.beta.kubernetes.io/zone"=$4
	kubectl label nodes $1 "topology.kubernetes.io/region"=$3
	kubectl label nodes $1 "topology.kubernetes.io/zone"=$4
}

function verify_node()
{
	kubectl get nodes | grep $1
	if (( $? == 0 ))
	then
		return 0
	else
		return 1
	fi
}

if (( $# < 4 ))
then
	help
fi

node=$1
instanceID=$2
region=$3
zone=$4

verify_node $node
if (( $? == 0 ))
then
	apply_labels $node $instanceID $region $zone
else
	echo "Node " \'$node\' " not found in the cluster, please check the node or passing correct parameters while executing script"
	help
fi
