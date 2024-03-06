# Introduction
This mutating webhook modifies a POD spec using specific runtimeclass to remove all `resources` entries and replace it with peer-pod extended resource.

## Need for mutating webhook
A peer-pod uses resources at two places:
- Kubernetes Worker Node: Peer-Pod metadata, Kata shim resources, remote-hypervisor/cloud-api-adaptor resources, vxlan etc
- Cloud Instance: The actual peer-pod VM running in the cloud (eg. EC2 instance in AWS, or Azure VM instance)

For peer-pods case the resources are really consumed outside of the worker node. It’s external to the Kubernetes cluster. 

This creates two problems:
1. Peer-pod scheduling can fail due to the unavailability of required resources on the worker node even though the peer-pod will not consume the requested resources from the worker node.

2. Cluster-admin have no way to view the actual peer-pods VM capacity and consumption.


A simple solution to the above problems is to advertise peer-pod capacity as Kubernetes extended resources and let Kubernetes scheduler handle the peer-pod capacity tracking and accounting. Additionally, POD overhead can be used to account for actual `cpu` and `mem` resource requirements on the Kubernetes worker node. 
The mutating webhook removes any `resources` entries from the Pod spec and adds the peer-pods extended resources.


![](https://i.imgur.com/MYwSQaX.png)



## Installation

Please refer to the following [instructions](docs/INSTALL.md)

## Development

Please refer to the following [instructions](docs/DEVELOPMENT.md)

