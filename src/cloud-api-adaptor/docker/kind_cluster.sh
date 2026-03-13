#!/bin/bash

# Ref: https://stackoverflow.com/questions/299728/how-do-you-use-newgrp-in-a-script-then-stay-in-that-group-when-the-script-exits
newgrp docker <<EOF

# Accept two arguments: create and delete
# create: creates a kind cluster
# delete: deletes a kind cluster

CLUSTER_NAME="${CLUSTER_NAME:-peer-pods}"
KIND_CONFIG_FILE="kind-config.yaml"

if [ "$1" == "create" ]; then
    # Check if kind is installed
    if [ ! -x "$(command -v kind)" ]; then
        echo "kind is not installed"
        exit 0
    fi
    echo "Check if the cluster \$CLUSTER_NAME already exists"
    if kind get clusters | grep -q "\$CLUSTER_NAME"; then
        echo "Cluster \$CLUSTER_NAME already exists"
        exit 0
    fi
    # Set some sysctls
    # Ref: https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
    sudo sysctl fs.inotify.max_user_watches=524288
    sudo sysctl fs.inotify.max_user_instances=512

    # Create a kind cluster
    echo "runtime: " "\$CONTAINER_RUNTIME"

    if [ "$CONTAINER_RUNTIME" == "crio" ]; then
        echo "Creating a kind cluster with crio runtime"
        KIND_CONFIG_FILE="kind-config-crio.yaml"
    fi

    # Create a kind cluster
    echo "Creating a kind cluster"
    kind create cluster --name "\$CLUSTER_NAME" --config "\$KIND_CONFIG_FILE" || exit 1

    # Deploy calico
    kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.31.0/manifests/calico.yaml || exit 1

    exit 0
fi

if [ "$1" == "delete" ]; then
    # Check if kind is installed
    if [ ! -x "$(command -v kind)" ]; then
        echo "kind is not installed"
        exit 0
    fi

    # Delete the kind cluster
    echo "Deleting the kind cluster"
    kind delete cluster --name "\$CLUSTER_NAME" || exit 1   

    exit 0
fi
EOF
