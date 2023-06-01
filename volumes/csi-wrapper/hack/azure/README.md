# csi-wrapper
Azure File CSI Wrapper for Peer Pod Storage

## High Level Design

![design](./images/csi-wrapper.png)

> **Note** Edited via https://excalidraw.com/

## Test CSI Wrapper with azurefiles-csi-driver for peer pod demo on Azure Cloud

### Set up a demo environment on your development machine

Follow the [README.md](../../azure/README.md) to setup a x86 based demo environment on Azure Cloud.

### Deploy azurefiles-csi-driver on the worker node
Note: All the steps can be performed anywhere with cluster access

1. Login to the worker node
```bash
ssh root@ip-of-your-worker-node
```

2. Clone the source code on the worker node:
```bash
git clone https://github.com/kubernetes-sigs/azurefile-csi-driver
cd azurefile-csi-driver
```
3. Get your worker instance id
- Check your worker node name
```bash
kubectl get nodes -o wide
```
4. Run the script:
```bash
bash ./deploy/install-driver.sh master local
```

### Deploy csi-wrapper to patch on azurefiles-csi-driver

1. Create the PeerpodVolume CRD object
```bash
cd /root/cloud-api-adaptor/
kubectl create -f volumes/csi-wrapper/crd/peerpodvolume.yaml
```
The output looks like:
```bash
customresourcedefinition.apiextensions.k8s.io/peerpodvolumes.peerpod.azure.com created
```
2. Create vpc-block-csi-wrapper-runner role  bind to azurefiles-controller-sa account
```bash
kubectl create -f volumes/csi-wrapper/hack/hack/azure-files-csi-wrapper-runner.yaml
```
3. Build csi-wrapper images on worker node:
```bash
apt install docker.io -y
cd volumes/csi-wrapper/
make import-csi-controller-wrapper-docker
make import-csi-node-wrapper-docker
make csi-podvm-wrapper-docker
```
4. Push csi-podvm-wrapper docker image to docker hub (Optional)
```bash
docker login -u [your_docker_hub_name] -p [your_docker_hub_password]
docker tag csi-podvm-wrapper:local [your_docker_hub_name]/csi-podvm-wrapper:dev
docker push [your_docker_hub_name]/csi-podvm-wrapper:dev
```
5. patch ibm-vpc-block-csi-driver:
```bash
kubectl patch rs csi-azurefile-controller-54c6fcbbc4 -n kube-system --patch-file hack/azure/patch-controller.yaml
kubectl patch ds csi-azurefile-node -n kube-system --patch-file hack/azure/patch-node.yaml
```

7. Create **storage class** for Peerpod:
```bash
kubectl apply -f hack/azyre/azure-file-StorageClass-for-peerpod.yaml.yaml
```

## Run the `csi-wrapper for peerpod storage` demo

1. Create one pvc that use `azurefiles-csi-driver`
```bash
kubectl create -f hack/azure/my-pvc-kube-system.yaml
```

2. Wait for the pvc status to become `bound`
```bash
3:54:25.693 [root@kartik-ThinkPad-X1-Titanium-Gen-1 csi-wrapper]# k get pvc -A
NAMESPACE     NAME            STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS         AGE
kube-system   pvc-azurefile   Bound    pvc-699ecee6-56f4-4a9e-9de6-72320c475504   1Gi        RWO            azure-file-storage   11h
```

3. Create nginx peer-pod demo with with `podvm-wrapper` and `azurefile-csi-driver` container
```bash
kubectl create -f hack/azure/nginx-kata-with-my-pvc-and-csi-wrapper.yaml
```
