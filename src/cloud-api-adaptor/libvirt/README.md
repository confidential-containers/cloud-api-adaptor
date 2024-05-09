# Cloud API Adaptor (CAA) on Libvirt

This document contains instructions for using, developing and testing the
cloud-api-adaptor with [libvirt](https://libvirt.org/).

You will learn how to setup an environment in your local machine to run peer
pods with the libvirt cloud API adaptor. Bear in mind that many different tools
can be used to setup the environment and here we just make suggestions of tools
that seems used by most of the peer pods developers.

# Prerequisites

You must have a Linux/KVM system with libvirt installed and the following tools:

- docker (or podman-docker)
- [kubectl](https://kubernetes.io/docs/reference/kubectl/)
- [kcli](https://kcli.readthedocs.io/en/latest/)
- [go](https://github.com/golang/go)

To configure the basic set of tools one can use
[config_libvirt.sh](config_libvirt.sh) script (tested on Ubuntu 20.04
amd64/s390x VSI) from the [cloud-api-adaptor](../) folder:

```bash
./libvirt/config_libvirt.sh
```

This script will: a) installs the dependencies above; b) ensure ``default``
libvirt storage pool is up and running; and c) configure the ssh keys and
libvirt uri.

> [!WARNING]
> The script provided is a utility for installing the specified packages using
> various methods and will modify your $HOME/.ssh folder. Please review the
> script thoroughly before running it. If you are not comfortable, you should
> install the prerequisites using your preferred method.

# Create the Kubernetes cluster

Use the [`kcli_cluster.sh`](./kcli_cluster.sh) script to create a simple two VMs (one control plane and one worker) cluster
with the kcli tool, as:

```bash
./libvirt/kcli_cluster.sh create
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
peer-pods-ctlplane-0 Ready    control-plane,master   6m8s    v1.26.7
peer-pods-worker-0   Ready    worker                 2m47s   v1.26.7
```

# Prepare the Pod VM volume

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

# Install and configure Confidential Containers and cloud-api-adaptor in the cluster

The easiest way to install the cloud-api-adaptor along with Confidential
Containers in the cluster is through the Kubernetes operator
[`install_operator.sh`](./install_operator.sh) script.

If you need to set any non-default parameter, please run the script with the
`--help` option.

```
$ export SSH_KEY_FILE="id_rsa"
$ ./libvirt/install_operator.sh
```

If everything goes well you will be able to see the operator's controller manager and cloud-api-adaptor Pods running:

```
$ kubectl get pods -n confidential-containers-system
NAME                                              READY   STATUS    RESTARTS   AGE
cc-operator-controller-manager-5df7584679-5dbmr   2/2     Running   0          3m58s
cloud-api-adaptor-daemonset-vgj2s                 1/1     Running   0          3m57s
$ kubectl logs pod/cloud-api-adaptor-daemonset-vgj2s -n confidential-containers-system
+ exec cloud-api-adaptor libvirt -uri 'qemu+ssh://wmoschet@192.168.122.1/system?no_verify=1' -data-dir /opt/data-dir -pods-dir /run/peerpod/pods -network-name default -pool-name default -socket /run/peerpod/hypervisor.sock
2022/11/09 18:18:00 [helper/hypervisor] hypervisor config {/run/peerpod/hypervisor.sock  registry.k8s.io/pause:3.7 /run/peerpod/pods libvirt}
2022/11/09 18:18:00 [helper/hypervisor] cloud config {qemu+ssh://wmoschet@192.168.122.1/system?no_verify=1 default default /opt/data-dir}
2022/11/09 18:18:00 [helper/hypervisor] service config &{qemu+ssh://wmoschet@192.168.122.1/system?no_verify=1 default default /opt/data-dir}
```

You will also notice that Kubernetes [*runtimeClass*](https://kubernetes.io/docs/concepts/containers/runtime-class/) resources
were created on the cluster, as for example:

```
$ kubectl get runtimeclass
NAME          HANDLER       AGE
kata-remote   kata-remote   7m18s
```

# Create a sample peer-pods pod

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
  runtimeClassName: kata-remote
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

# Running the CAA e2e tests

Now when you're all set you can run the CAA e2e [tests/e2e/README.md](../test/e2e/README.md) by running ``make test-e2e``. You might want to modify some of the env variables, for example:

```
make TEST_PROVISION=no TEST_TEARDOWN=no TEST_PODVM_IMAGE=$PWD/podvm/podvm.qcow2 CLOUD_PROVIDER=libvirt TEST_E2E_TIMEOUT=40m TEST_PROVISION_FILE=$PWD/libvirt.properties test-e2e
```

* ``TEST_PROVISION`` - whether to perform the setup steps (we just did thated for it)
* ``TEST_TEARDOWN`` - attempt to clean all resources created by `test-e2e` after testing (no guarantee)
* ``TEST_PODVM_IMAGE`` - image to be used for this testing
* ``CLOUD_PROVIDER`` - which cloud provider should be used
* ``TEST_E2E_TIMEOUT`` - test timeout
* ``DEPLOY_KBS`` - whether to deploy the key-broker-service, which is used to test the attestation flow
* ``TEST_PROVISION_FILE`` - file specifying the libvirt connection and the ssh key file (created earlier by [config_libvirt.sh](config_libvirt.sh))

# Delete Confidential Containers and cloud-api-adaptor from the cluster

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

In order to remove the *\*-cloud-api-adaptor-daemonset-\** pod, run the following command from the
root directory:

```
$ CLOUD_PROVIDER=libvirt make delete

kubectl delete -k install/overlays/libvirt
serviceaccount "cloud-api-adaptor" deleted
clusterrole.rbac.authorization.k8s.io "node-viewer" deleted
clusterrole.rbac.authorization.k8s.io "peerpod-editor" deleted
clusterrole.rbac.authorization.k8s.io "pod-viewer" deleted
clusterrolebinding.rbac.authorization.k8s.io "node-viewer" deleted
clusterrolebinding.rbac.authorization.k8s.io "peerpod-editor" deleted
clusterrolebinding.rbac.authorization.k8s.io "pod-viewer" deleted
configmap "peer-pods-cm" deleted
secret "auth-json-secret" deleted
secret "peer-pods-secret" deleted
secret "ssh-key-secret" deleted
daemonset.apps "cloud-api-adaptor-daemonset" deleted
```

This can be useful if one needs to update kustomization.yaml. After making changes, one can re-apply the cloud-api-adaptor with:
```
kubectl apply -k install/overlays/libvirt/
```

To delete Confidential Containers, (the ccruntime resource, the cc-operator-daemon-install and cc-operator-pre-install-daemon pods) run:

```
$ kubectl delete -k "github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods"

ccruntime.confidentialcontainers.org "ccruntime-peer-pods" deleted
```

It can take some minutes to get those pods deleted. Afterwards you will notice that only the *controller-manager* pod is
still up. The ccruntime resource will also be deleted. This can be verified with:
```
kubectl get ccruntime
```
which should return nothing.

To delete the *controller-manager*:

```
$ kubectl delete -k "github.com/confidential-containers/operator/config/default"

namespace "confidential-containers-system" deleted
customresourcedefinition.apiextensions.k8s.io "ccruntimes.confidentialcontainers.org" deleted
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
```

Verify that all pods and the namespace has been destroyed.
```
kubectl get pods -n confidential-containers-system
```
should return nothing. Additionally,

```
kubectl get namespaces
```
should show that there is no longer a confidential-containers-system namespace.
