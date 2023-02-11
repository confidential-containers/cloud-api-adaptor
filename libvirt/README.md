# Introduction

This document contains instructions for using, developing and testing the cloud-api-adaptor with [libvirt](https://libvirt.org/).

# Creating an end-to-end environment for testing and development

In this section you will learn how to setup an environment in your local machine to run peer pods with
the libvirt cloud API adaptor. Bear in mind that many different tools can be used to setup the environment
and here we just make suggestions of tools that seems used by most of the peer pods developers.

## Requirements

You must have a Linux/KVM system with libvirt installed and the following tools:

- docker (or podman-docker)
- [kubectl](https://kubernetes.io/docs/reference/kubectl/)
- [kcli](https://kcli.readthedocs.io/en/latest/)

Assume that you have a 'default' network and storage pools created in libvirtd system instance (`qemu:///system`). However,
if you have a different pool name then the scripts should be able to handle it properly.

## Create the Kubernetes cluster

Use the [`kcli_cluster.sh`](./kcli_cluster.sh) script to create a simple two VMs (one control plane and one worker) cluster
with the kcli tool, as:

```
./kcli_cluster.sh create
```

With `kcli_cluster.sh` you can configure the libvirt network and storage pools that the cluster VMs will be created, among
other parameters. Run `./kcli_cluster.sh -h` to see the help for further information.

If everything goes well you will be able to see the cluster running after setting your Kubernetes config with:

`export KUBECONFIG=$HOME/.kcli/clusters/peer-pods/auth/kubeconfig`

For example, shown below:

```
$ kcli list kube
+-----------+---------+-----------+-----------------------------------------+
|  Cluster  |   Type  |    Plan   |                  Vms                    |
+-----------+---------+-----------+-----------------------------------------+
| peer-pods | generic | peer-pods | peer-pods-ctlplane-0,peer-pods-worker-0 |
+-----------+---------+-----------+-----------------------------------------+
$ kubectl get nodes
NAME                 STATUS   ROLES                  AGE     VERSION
peer-pods-ctlplane-0 Ready    control-plane,master   6m8s    v1.25.3
peer-pods-worker-0   Ready    worker                 2m47s   v1.25.3
```

## Prepare the Pod VM volume

In order to build the Pod VM without installing the build tools, you can use the Dockerfiles hosted on [../podvm](../podvm) directory to run the entire process inside a container. Refer to [podvm/README.md](../podvm/README.md) for further details. Alternatively you can consume pre-built podvm images as explained [here](../docs/consuming-prebuilt-podvm-images.md).

Next you will need to create a volume on libvirt's system storage and upload the image content. That volume is used by
the cloud-api-adaptor program to instantiate a new Pod VM. Run the following commands:

```
$ export IMAGE=<full-path-to-qcow2>

$ virsh -c qemu:///system vol-create-as --pool default --name podvm-base.qcow2 --capacity 20G --allocation 2G --prealloc-metadata --format qcow2
$ virsh -c qemu:///system vol-upload --vol podvm-base.qcow2 $IMAGE --pool default --sparse
```

You should see that the `podvm-base.qcow2` volume was properly created:

```
$ virsh -c qemu:///system vol-info --pool default podvm-base.qcow2
Name:           podvm-base.qcow2
Type:           file
Capacity:       6.00 GiB
Allocation:     631.52 MiB
```

## Install and configure Confidential Containers and cloud-api-adaptor in the cluster

The easiest way to install the cloud-api-adaptor along with Confidential Containers in the cluster is through the
Kubernetes operator available in the `install` directory of this repository.

Start by creating a public/private RSA key pair that will be used by the cloud-api-provider program, running on the
cluster workers, to connect with your local libvirtd instance without password authentication. Assume you are in the
`libvirt` directory, do:

```
$ cd ../install/overlays/libvirt
$ ssh-keygen -f ./id_rsa -N ""
$ cat id_rsa.pub >> ~/.ssh/authorized_keys
```
**Note**: ensure that `~/.ssh/authorized_keys` has the right permissions (read/write for the user only) otherwise the
authentication can silently fail. You can run `chmod 600 ~/.ssh/authorized_keys` to set the right permissions.

You will need to figure out the IP address of your local host (e.g. 192.168.122.1). Then try to remote connect with
libvirt to check the keys setup is fine, for example:

```
$ virsh -c "qemu+ssh://$USER@192.168.122.1/system?keyfile=$(pwd)/id_rsa" nodeinfo
CPU model:           x86_64
CPU(s):              12
CPU frequency:       1084 MHz
CPU socket(s):       1
Core(s) per socket:  6
Thread(s) per core:  2
NUMA cell(s):        1
Memory size:         32600636 KiB
```

Now you should finally install the Kubernetes operator in the cluster with the help of the [`install_operator.sh`](./install_operator.sh) script. Ensure that you have your IP address exported in the environment, as shown below, then run the install script:

```
$ cd ../../../libvirt/
$ export LIBVIRT_IP="192.168.122.1"
$ export SSH_KEY_FILE="id_rsa"
$ ./install_operator.sh
```

If everything goes well you will be able to see the operator's controller manager and cloud-api-adaptor Pods running:

```
$ kubectl get pods -n confidential-containers-system
NAME                                              READY   STATUS    RESTARTS   AGE
cc-operator-controller-manager-5df7584679-5dbmr   2/2     Running   0          3m58s
cloud-api-adaptor-daemonset-libvirt-vgj2s         1/1     Running   0          3m57s
$ kubectl logs pod/cloud-api-adaptor-daemonset-libvirt-vgj2s -n confidential-containers-system
+ exec cloud-api-adaptor-libvirt libvirt -uri 'qemu+ssh://wmoschet@192.168.122.1/system?no_verify=1' -data-dir /opt/data-dir -pods-dir /run/peerpod/pods -network-name default -pool-name default -socket /run/peerpod/hypervisor.sock
2022/11/09 18:18:00 [helper/hypervisor] hypervisor config {/run/peerpod/hypervisor.sock  k8s.gcr.io/pause:3.7 /run/peerpod/pods libvirt}
2022/11/09 18:18:00 [helper/hypervisor] cloud config {qemu+ssh://wmoschet@192.168.122.1/system?no_verify=1 default default /opt/data-dir}
2022/11/09 18:18:00 [helper/hypervisor] service config &{qemu+ssh://wmoschet@192.168.122.1/system?no_verify=1 default default /opt/data-dir}
```

You will also notice that Kubernetes [*runtimeClass*](https://kubernetes.io/docs/concepts/containers/runtime-class/) resources
were created on the cluster, as for example:

```
$ kubectl get runtimeclass
NAME        HANDLER     AGE
kata        kata        7m41s
kata-clh    kata-clh    7m41s
kata-qemu   kata-qemu   7m41s
```

## Create a sample peer-pods pod

At this point everything should be fine to get a sample Pod created. Let's first list the running VMs so that we can later check
the Pod VM will be really running. Notice below that we got only the cluster node VMs up:

```
$ virsh -c qemu:///system list
 Id   Name                   State
------------------------------------
 3    peer-pods-ctlplane-0   running
 4    peer-pods-worker-0     running
```

Create the *sample_busybox.yaml* file with the following content:

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    run: busybox
  name: busybox
spec:
  containers:
  - image: quay.io/prometheus/busybox
    name: busybox
    resources: {}
  dnsPolicy: ClusterFirst
  restartPolicy: Never
  runtimeClassName: kata
```

And create the Pod:

```
$ kubectl apply -f sample_busybox.yaml
pod/busybox created
$ kubectl wait --for=condition=Ready pod/busybox
pod/busybox condition met
```

Check that the Pod VM is up and running. See on the following listing that *podvm-busybox-88a70031* was
created:

```
$ virsh -c qemu:///system list
 Id   Name                     State
----------------------------------------
 5    peer-pods-ctlplane-0     running
 6    peer-pods-worker-0       running
 7    podvm-busybox-88a70031   running
```

You should also check that the container is running fine. For example, compare the kernels are different as shown below:

```
$ uname -r
5.17.12-100.fc34.x86_64
$ kubectl exec pod/busybox -- uname -r
5.4.0-131-generic
```

The peer-pods pod can be deleted as any regular pod. On the listing below the pod was removed and you can note that the
Pod VM no longer exists on Libvirt:

```
$ kubectl delete -f sample_busybox.yaml
pod "busybox" deleted
$ virsh -c qemu:///system list
 Id   Name                 State
------------------------------------
 5    peer-pods-ctlplane-0 running
 6    peer-pods-worker-0   running
```

## Delete Confidential Containers and cloud-api-adaptor from the cluster

You might want to reinstall the Confidential Containers and cloud-api-adaptor into your cluster. There are two options:

1. Delete the Kubernetes cluster entirely and start over. In this case you should just run `./kcli_cluster.sh delete` to
   wipe out the cluster created with kcli
1. Uninstall the operator resources then install them again with the `install_operator.sh` script

Let's show you how to delete the operator resources. On the listing below you can see the actual pods running on
the *confidential-containers-system* namespace:

```
$ kubectl get pods -n confidential-containers-system
NAME                                             READY   STATUS    RESTARTS   AGE
cc-operator-controller-manager-fbb5dcf9d-h42nn   2/2     Running   0          20h
cc-operator-daemon-install-fkkzz                 1/1     Running   0          20h
cloud-api-adaptor-daemonset-libvirt-lxj7v        1/1     Running   0          20h
```

In order to remove the *\*-daemon-install-\** and *\*-cloud-api-adaptor-daemonset-\** pods, run the following command from the
root directory:

```
$ CLOUD_PROVIDER=libvirt make delete
```

It can take some minutes to get those pods deleted, afterwards you will notice that only the *controller-manager* is
still up. Below is shown how to delete that pod and associated resources as well:

```
$ kubectl get pods -n confidential-containers-system
NAME                                             READY   STATUS    RESTARTS   AGE
cc-operator-controller-manager-fbb5dcf9d-h42nn   2/2     Running   0          20h
$ kubectl delete -f install/yamls/deploy.yaml
namespace "confidential-containers-system" deleted
serviceaccount "cc-operator-controller-manager" deleted
role.rbac.authorization.k8s.io "cc-operator-leader-election-role" deleted
clusterrole.rbac.authorization.k8s.io "cc-operator-manager-role" deleted
clusterrole.rbac.authorization.k8s.io "cc-operator-metrics-reader" deleted
clusterrole.rbac.authorization.k8s.io "cc-operator-proxy-role" deleted
rolebinding.rbac.authorization.k8s.io "cc-operator-leader-election-rolebinding" deleted
clusterrolebinding.rbac.authorization.k8s.io "cc-operator-manager-rolebinding" deleted
clusterrolebinding.rbac.authorization.k8s.io "cc-operator-proxy-rolebinding" deleted
configmap "cc-operator-manager-config" deleted
service "cc-operator-controller-manager-metrics-service" deleted
deployment.apps "cc-operator-controller-manager" deleted
customresourcedefinition.apiextensions.k8s.io "ccruntimes.confidentialcontainers.org" deleted
$ kubectl get pods -n confidential-containers-system
No resources found in confidential-containers-system namespace.
```
