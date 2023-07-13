# Test CSI Wrapper with ibm-vpc-block-csi-driver for peer pod demo on IBM Cloud VPC

## Set up a demo environment on your development machine

Follow the [README.md](../../../../ibmcloud/README.md) to setup a x86_64 based demo environment on IBM Cloud VPC.

## Deploy `ibm-vpc-block-csi-driver` on the cluster

1. Setup kubeconfig so that you can access the cluster using `kubectl`.

2. Get the worker instance id
- Check the worker node name
```bash
kubectl get nodes -o wide
```
- Check vsi instance id for the worker node
```bash
export IBMCLOUD_API_KEY=<your_api_key>
/root/cloud-api-adaptor/ibmcloud/image/login.sh

```
> **Note** You can export `IBMCLOUD_API_ENDPOINT` and `IBMCLOUD_VPC_REGION` to use other regions
- List instances
```bash
ibmcloud is ins |grep ${your-worker-node-name}
```
The expected result should look like:
```bash
...
0797_162a604f-82da-4b8c-9144-1204d4c560db   liudali-csi-amd64-node-1                   running   10.242.64.19   141.125.156.56    bx2-2x8     ibm-ubuntu-20-04-3-minimal-amd64-1                se-image-e2e-test-eu-gb        eu-gb-2   Default
```
In the example `0797_162a604f-82da-4b8c-9144-1204d4c560db` is the instanceID, `liudali-csi-amd64-node-1` is the node-name, `eu-gb-2` is the zone, the region should be `eu-gb`.

3. Set providerID with correct instanceID:
`kubectl patch node {the-worker-node-name} -p '{"spec":{"providerID":"//////${instanceID-of-the-worker-node}"}}'`
eg:
```
kubectl patch node liudali-csi-amd64-node-1 -p '{"spec":{"providerID":"//////0797_162a604f-82da-4b8c-9144-1204d4c560db"}}'
node/liudali-csi-amd64-node-1 patched
```
> **Note**
> - The `providerID` for node only can be set once, if you want to update it's value again you will get follow error:
> `The Node "liudali-csi-amd64-node-1" is invalid: spec.providerID: Forbidden: node updates may not change providerID except from "" to valid`.
>
> The only way is delete the node from your cluster and then run the `kubeadm join` command again.
> - On control panel:
> ```
> kubectl delete node liudali-csi-amd64-node-1
> kubeadm token create --print-join-command
> ```
> - On the worker node, clean env and run the join command from above output:
> ```
> kubeadm reset
> rm -rf /etc/cni/net.d
> kubeadm join 10.242.64.18:6443 --token twdudr.gkoyhki9915qh7a8 --discovery-token-ca-cert-hash sha256:445b3a713ede97dd4a0d62fafd4dee6bba0b2d42c3cb4db5fc0df215b3fd5542
> reboot
> ```
> After the worker node status changed to ready, please set the work role again:
> `kubectl label node liudali-csi-amd64-node-1 node.kubernetes.io/worker=`

4. Add labels to worker node:
```bash
cd /root/cloud-api-adaptor/volumes/csi-wrapper/
bash ./hack/ibm/apply-required-labels.sh <node-name> <instanceID> <region-of-instanceID> <zone-of-instanceID>
```
The expected result looks like:
```bash
bash ./hack/ibm/apply-required-labels.sh liudali-csi-amd64-node-1 0797_162a604f-82da-4b8c-9144-1204d4c560db eu-gb eu-gb-2
liudali-csi-amd64-node-1   Ready    worker          2h   v1.27.3
node/liudali-csi-amd64-node-1 labeled
node/liudali-csi-amd64-node-1 labeled
node/liudali-csi-amd64-node-1 labeled
node/liudali-csi-amd64-node-1 labeled
node/liudali-csi-amd64-node-1 labeled
```

5. Create the [slclient_Gen2.toml](https://github.com/kubernetes-sigs/ibm-vpc-block-csi-driver/blob/v5.2.0/deploy/kubernetes/driver/kubernetes/slclient_Gen2.toml) for the cluster:
```bash
export IBMCLOUD_VPC_REGION=<the_region_name>
export IBMCLOUD_RESOURCE_GROUP_ID=<check via `ibmcloud resource groups`>
export IBMCLOUD_API_KEY=<your ibm cloud API key>
cat <<END > slclient_Gen2.toml
[VPC]
  iam_client_id = "bx"
  iam_client_secret = "bx"
  g2_token_exchange_endpoint_url = "https://iam.cloud.ibm.com"
  g2_riaas_endpoint_url = "https://${IBMCLOUD_VPC_REGION}.iaas.cloud.ibm.com"
  g2_resource_group_id = "${IBMCLOUD_RESOURCE_GROUP_ID}"
  g2_api_key = "${IBMCLOUD_API_KEY}"
  provider_type = "g2"
END
cat slclient_Gen2.toml
```
> **Note** Please export `IBMCLOUD_VPC_REGION`, `IBMCLOUD_RESOURCE_GROUP_ID`, `IBMCLOUD_API_KEY` with correct values and check what the file was updated.

6. Deploy original `ibm-vpc-block-csi-driver`:
```bash
encodeVal=$(base64 -w 0 slclient_Gen2.toml)
sed -i "s/REPLACE_ME/$encodeVal/g" ./hack/ibm/ibm-vpc-block-csi-driver-v5.2.0.yaml
kubectl create -f ./hack/ibm/ibm-vpc-block-csi-driver-v5.2.0.yaml
```
Check `ibm-vpc-block-csi-driver related` pod status
```bash
kubectl get po -A -o wide | grep vpc
kube-system                      ibm-vpc-block-csi-controller-bdfdf4657-vksh2       6/6     Running            0             15s
kube-system                      ibm-vpc-block-csi-node-2w4jf                       2/3     CrashLoopBackOff   1 (12s ago)   16s
kube-system                      ibm-vpc-block-csi-node-pf2j6                       3/3     Running            0             16s
```
> **Note**
> - The `CrashLoopBackOff` for node plugin `ibm-vpc-block-csi-node-2w4jf` on controller node is expected

## Deploy csi-wrapper to patch on ibm-vpc-block-csi-driver

1. Create the PeerpodVolume CRD object
```bash
kubectl create -f crd/peerpodvolume.yaml
```
The output looks like:
```bash
customresourcedefinition.apiextensions.k8s.io/peerpodvolumes.confidentialcontainers.org created
```
2. Create vpc-block-csi-wrapper-runner role  bind to ibm-vpc-block-controller-sa account
```bash
kubectl create -f hack/ibm/vpc-block-csi-wrapper-runner.yaml
```
The output looks like:
```
clusterrole.rbac.authorization.k8s.io/vpc-block-csi-wrapper-runner created
clusterrolebinding.rbac.authorization.k8s.io/vpc-block-csi-wrapper-controller-binding created
clusterrolebinding.rbac.authorization.k8s.io/vpc-block-csi-wrapper-node-binding created
```

3. patch ibm-vpc-block-csi-driver:
```bash
kubectl patch Deployment ibm-vpc-block-csi-controller -n kube-system --patch-file hack/ibm/patch-controller.yaml
kubectl patch ds ibm-vpc-block-csi-node -n kube-system --patch-file hack/ibm/patch-node.yaml
```

4. Check pod status now:
```bash
kubectl get po -A -o wide | grep vpc
kube-system                      ibm-vpc-block-csi-controller-664ccf487d-q9zjh      7/7     Running             0             22s     172.20.3.14    liudali-csi-amd64-node-1   <none>           <none>
kube-system                      ibm-vpc-block-csi-node-fc8lx                       4/4     Running             0             21s     172.20.3.15    liudali-csi-amd64-node-1   <none>           <none>
kube-system                      ibm-vpc-block-csi-node-qbfhc                       0/4     ContainerCreating   0             21s     <none>         liudali-csi-amd64-node-0   <none>           <none>
```
> **Note** The `ibm-vpc-block-csi-node-qbfhc` pod won't in `Running` status as it's on control node, just ignore it.

5. Create **storage class** for Peerpod:
```bash
kubectl apply -f hack/ibm/ibm-vpc-block-5iopsTier-StorageClass-for-peerpod.yaml
```
> **Note**
> - One parameter `peerpod: "true"` is added, without it, csi-wrapper won't create PeerpodVolume objects, csi-requests will be processed as normal csi-requests.

# Run the `csi-wrapper for peerpod storage` demo

1. Create one pvc that use the storage class for peerpod.
```bash
kubectl create -f hack/ibm/my-pvc-kube-system.yaml
```

2. Wait for the pvc status to become `bound`
```bash
kubectl -n kube-system get pvc
NAME     STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS                AGE
my-pvc   Bound    pvc-b0803078-551e-42bc-9a44-fc98d95b8010   10Gi       RWO            ibmc-vpc-block-5iops-tier   58s
```

3. Create nginx peer-pod demo with with `podvm-wrapper` and `ibm-vpc-block-csi-driver` containers
```bash
kubectl create -f hack/ibm/nginx-kata-with-my-pvc-and-csi-wrapper.yaml
```

4. Wait 2 minutes, check if the `nginx` pod is running:
```bash
kubectl get po -A | grep nginx
kube-system   nginx                                                 3/3     Running             0              101s
```

5. Check the `PeerpodVolume` crd object and the state is `nodePublishVolumeApplied`
- Getting `PeerpodVolume` objects:
```bash
kubectl get PeerpodVolume -A
NAMESPACE     NAME                                        AGE
kube-system   r018-1f874ae2-abed-4f37-853e-b1748acd8ced   2m35s
```
- Using name get the state of the PeerpodVolume:
```bash
kubectl -n kube-system get PeerpodVolume r018-1f874ae2-abed-4f37-853e-b1748acd8ced -o yaml |grep state
  state: nodePublishVolumeApplied
```

6. Exec into nginx container check the mount:
```bash
kubectl exec nginx -n kube-system -c nginx -i -t -- sh
# lsblk
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINT
loop0     7:0    0 91.8M  1 loop
loop1     7:1    0 63.2M  1 loop
loop2     7:2    0 49.6M  1 loop
vda     252:0    0  100G  0 disk
|-vda1  252:1    0 99.9G  0 part /run/secrets/kubernetes.io/serviceaccount
|-vda14 252:14   0    4M  0 part
`-vda15 252:15   0  106M  0 part
vdb     252:16   0  372K  0 disk
vdc     252:32   0   44K  0 disk
vdd     252:48   0   10G  0 disk /mount-path
# ls /mount-path
lost+found
```
> **Note** We can see, the device **vdd** is mounted to `/mount-path` in nginx container as expected.

- Create a file under `/mount-path`
```bash
echo "from nignx container date:" $(date) > /mount-path/incontainer.log
```
- On the development machine check the instanceID of created vsi for nginx pod:
```bash
ibmcloud is ins |grep nginx
0797_aa4084b3-253b-4720-8f90-152957b29410   podvm-nginx-b3593f0a                       running   10.242.64.4    -                 bx2-2x8     podvm-e2e-test-image-amd64                        se-image-e2e-test-eu-gb        eu-gb-2   Default
```
- Create a floating ip for the nginx pod VSI
```bash
export vsi_name=podvm-nginx-b3593f0a
export vsi_id=0797_aa4084b3-253b-4720-8f90-152957b29410
nic_id=$(ibmcloud is instance ${vsi_id} --output JSON | jq -r '.network_interfaces[].id')
floating_ip=$(ibmcloud is floating-ip-reserve ${vsi_name}-ip --nic-id ${nic_id} --output JSON | jq -r '.address')
echo $floating_ip
141.125.163.10
```
- Login to the created vsi
```bash
ssh root@141.125.163.10
```
- Check the mount point:
```bash
root@podvm-nginx-b3593f0a:~# lsblk
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINT
loop0     7:0    0 63.5M  1 loop /snap/core20/1891
loop1     7:1    0 91.9M  1 loop /snap/lxd/24061
loop2     7:2    0 53.3M  1 loop /snap/snapd/19361
vda     252:0    0   10G  0 disk
├─vda1  252:1    0  9.9G  0 part /
├─vda14 252:14   0    4M  0 part
└─vda15 252:15   0  106M  0 part /boot/efi
vdb     252:16   0  374K  0 disk
vdc     252:32   0   44K  0 disk
vdd     252:48   0   10G  0 disk /var/lib/kubelet/pods/d57f98fe-b6b3-48bd-900f-d565d2823032/volumes/kubernetes.io~csi/pvc-b0803078-551e-42bc-9a44-fc98d95b8010/mount
root@podvm-nginx-b3593f0a:~#
```
> **Note** We can see, the device **vdd** is mounted to `/var/lib/kubelet/pods/d57f98fe-b6b3-48bd-900f-d565d2823032/volumes/kubernetes.io~csi/pvc-b0803078-551e-42bc-9a44-fc98d95b8010/mount` in the vsi as expected.

- Check the created file from container
```
cat /var/lib/kubelet/pods/d57f98fe-b6b3-48bd-900f-d565d2823032/volumes/kubernetes.io~csi/pvc-b0803078-551e-42bc-9a44-fc98d95b8010/mount/incontainer.log
from nignx container date: Tue Jun 20 07:40:08 UTC 2023
```

## Debug
- Monitor the csi-controller-wrapper log:
```
kubectl logs -n kube-system ibm-vpc-block-csi-controller-664ccf487d-q9zjh -c csi-controller-wrapper -f
```
- Monitor the cloud-api-adaptor-daemonset log:
```
kubectl logs -n confidential-containers-system cloud-api-adaptor-daemonset-gx69f -f
```
