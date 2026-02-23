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
- [helm](https://helm.sh)

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
peer-pods-ctlplane-0 Ready    control-plane,master   6m8s    v1.30.0
peer-pods-worker-0   Ready    worker                 2m47s   v1.30.0
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
Containers in the cluster is through the [`install_caa.sh`](./install_caa.sh)
script.

If you need to set any non-default parameter, please run the script with the
`--help` option.

```
$ export SSH_KEY_FILE="${HOME}/.ssh/id_rsa"
$ ./libvirt/install_caa.sh
```

If everything goes well you will be able to see the kata-deploy and cloud-api-adaptor Pods running:

```
$ kubectl get pods -n confidential-containers-system
NAME                                              READY   STATUS    RESTARTS   AGE
cloud-api-adaptor-daemonset-72xm5                 1/1     Running   0          3m39s
kata-deploy-mbc6c                                 1/1     Running   0          3m39s
peerpodctrl-controller-manager-74b5bb8c8b-76glp   2/2     Running   0          3m39s
$ kubectl logs -l app=cloud-api-adaptor -n confidential-containers-system --tail=-1
+ exec cloud-api-adaptor libvirt -data-dir /opt/data-dir
2026/02/23 17:31:13 [adaptor/cloud] Cloud provider external plugin loading is disabled, skipping plugin loading
2026/02/23 17:31:13 [adaptor/cloud/libvirt] libvirt config: &libvirt.Config{URI:"qemu+ssh://wmoschet@192.168.122.1/system?no_verify=1", PoolName:"default", NetworkName:"default", DataDir:"/opt/data-dir", DisableCVM:true, VolName:"podvm-base.qcow2", LaunchSecurity:"", Firmware:"/usr/share/OVMF/OVMF_CODE_4M.fd", CPU:0x2, Memory:0x2000}
cloud-api-adaptor version v0.17.0-dev
  commit: b7569dc107c7b36487903641e493c1852056d574
  go: go1.24.13
cloud-api-adaptor: starting Cloud API Adaptor daemon for "libvirt"
2026/02/23 17:31:17 [adaptor/cloud/libvirt] Created libvirt connection
2026/02/23 17:31:17 [adaptor] server config: &cloud.ServerConfig{TLSConfig:(*tlsutil.TLSConfig)(0xc0003de300), SocketPath:"/run/peerpod/hypervisor.sock", PauseImage:"", PodsDir:"/run/peerpod/pods", ForwarderPort:"15150", ProxyTimeout:300000000000, Initdata:"", EnableCloudConfigVerify:false, PeerPodsLimitPerNode:10, RootVolumeSize:0, EnableScratchSpace:false}
2026/02/23 17:31:17 [util/k8sops] initialized PeerPodService
2026/02/23 17:31:17 [probe/probe] Using port: 8000
2026/02/23 17:31:17 [util/k8sops] set up extended resources
2026/02/23 17:31:17 [util/k8sops] Successfully set extended resource for node peer-pods-worker-0
2026/02/23 17:31:17 [adaptor] server started
2026/02/23 17:31:41 [probe/probe] nodeName: peer-pods-worker-0
2026/02/23 17:31:41 [probe/probe] Selected pods count: 12
2026/02/23 17:31:41 [probe/probe] All PeerPods standup. we do not check the PeerPods status any more.
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
1. Uninstall the Peerpods helm chart then install them again with the `install_caa.sh` script

Let's show you how to delete the helm chart. On the listing below you can see the actual pods running on
the *confidential-containers-system* namespace:

```
$ kubectl get pods -n confidential-containers-system
NAME                                              READY   STATUS    RESTARTS   AGE
cloud-api-adaptor-daemonset-z7qwt                 1/1     Running   0          9m34s
kata-deploy-6hgdq                                 1/1     Running   0          9m34s
peerpodctrl-controller-manager-74b5bb8c8b-rrsmd   2/2     Running   0          9m34s
```

In order to remove all pods, run the following command from the
root directory:

```
$ CLOUD_PROVIDER=libvirt make delete
helm uninstall peerpods -n confidential-containers-system
release "peerpods" uninstalled
```

Verify that all pods and the namespace has been destroyed.
```
kubectl get pods -n confidential-containers-system
```
should return nothing.

This can be useful if one needs to update [install/charts/peerpods/providers/libvirt.yaml](../install/charts/peerpods/providers/libvirt.yaml). After making changes, one can re-apply the cloud-api-adaptor with:
```
CLOUD_PROVIDER=libvirt make deploy
```
