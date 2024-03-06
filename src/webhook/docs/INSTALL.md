## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.

### Deploy extended resources

You'll need to advertise the extended resource `kata.peerpods.io/vm`.

A simple daemonset is provided under the following [directory](../hack/extended-resources/).

The following command advertises `20` pod VM resources. You can modify the spec as needed.

```
cd ../hack/extended-resources
./setup.sh
```

To verify, check the node object
```
kubectl get node $NODENAME -o=jsonpath='{.status.allocatable}' | jq
```

You should see a similar output like the one below:
```
{
  "attachable-volumes-aws-ebs": "25",
  "cpu": "2",
  "ephemeral-storage": "56147256023",
  "hugepages-1Gi": "0",
  "hugepages-2Mi": "0",
  "kata.peerpods.io/vm": "20",
  "memory": "7935928Ki",
  "pods": "110"
}
```

### Using kind cluster
For `kind` clusters, you can use the following Makefile targets

Create kind cluster
```
make kind-cluster
```
Deploy the webhook in the kind cluster
```
make kind-deploy IMG=quay.io/confidential-containers/peer-pods-webhook
```

### Using a remote cluster

#### Deploy cert-manager
```
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.9.1/cert-manager.yaml
```

#### Deploy webhook
```
kubectl apply -f hack/webhook-deploy.yaml
```

The default `RuntimeClass` that the webhook monitors is `kata-remote`.
The default `RuntimeClass` can be changed by modifying the `TARGET_RUNTIMECLASS` environment variable.
For example, executing the following command changes it to `kata-remote`

```
kubectl set env deployment/peer-pods-webhook-controller-manager -n peer-pods-webhook-system TARGET_RUNTIMECLASS=kata-remote
```

The default Pod VM instance type is `t2.small` and can be changed by modifying the `POD_VM_INSTANCE_TYPE` environment variable.
