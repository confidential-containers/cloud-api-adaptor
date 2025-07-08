# Introduction

Currently, all pod traffic is routed via the worker node (WN).
This is by design and is fine as all existing network policies that were applicable to pods running on the WN also apply to peer pods.
However, the peer pods approach has enabled some additional use cases that were earlier impossible or impractical. Concurrently, these new use cases have resulted in new requirements.

Consider a scenario where you are running a Kind K8s cluster on your laptop, experimenting with an AI model, and needing additional computing power or GPUs. With peer pods, you can spin up a pod using the appropriate instance type from the Kind K8s cluster running on your laptop.

From a networking standpoint, this means that there should be bidirectional connectivity between the cloud and your development setup.

One solution is to use a VPN, which complicates the setup. A relatively easier alternative is to use a public IP associated with the cloud instance for control-plane traffic and use the cloud instance network for external connectivity requirements (like accessing cloud object storage for data) instead of routing all traffic via your development setup.

The external network connectivity for the pod via pod VM network is enabled via the
option `ext-network-via-podvm` in cloud-api-adaptor. The equivalent option in the `peer-pods-cm` is `EXTERNAL_NETWORK_VIA_PODVM`

The prerequisite is for the pod VM to have a secondary interface with an IP. This interface will be moved to the pod network namespace and default routes adjusted so that pod network traverses via worker node, and any other traffic uses the secondary interface.

**This is experimental feature and currently only available for AWS and Alibaba Cloud**

## Specifying Pod subnet CIDRs

When using the `ext-network-via-podvm` feature, the default route for the pod is set to the secondary interface. Traffic on the pod subnet is routed via the pod network specific interface (vxlan). The pod subnet is auto detected based on the pod IP by Linux for the route. This may not be sufficient for all cases and you may want to specify pod subnets explicitly.
We have an option for the same named `pod-subnet-cidrs` to provide a comma separated list of CIDRs that will be routed via the pod network specific interface.

The pod subnet CIDR, service CIDR depends on Kubernetes CNI configuration. Here are some examples:

* A generic way to detect the cluster service CIDR as mentioned [here](Ref: https://stackoverflow.com/questions/44190607/how-do-you-find-the-cluster-service-cidr-of-a-kubernetes-cluster)

```sh
SVCRANGE=$(echo '{"apiVersion":"v1","kind":"Service","metadata":{"name":"tst"},"spec":{"clusterIP":"1.1.1.1","ports":[{"port":443}]}}' | kubectl apply -f - 2>&1 | sed 's/.*valid IPs is //')
echo $SVCRANGE
```

* The default pod CIDR for Flannel CNI: `10.244.0.0/16`
* AWS VPC CNI uses VPC CIDR as the pod CIDR
* Azure CNI uses the subnet CIDR as the pod CIDR
* The default pod CIDR for OVN-Kubernetes: `10.128.0.0/14`