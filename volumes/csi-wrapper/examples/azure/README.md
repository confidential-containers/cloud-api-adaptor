# Azure File CSI Wrapper for Peer Pod Storage

## Peer Pod example using CSI Wrapper with azurefiles-csi-driver

### Set up a demo environment on your development machine

1. Follow the [README.md](../../../../azure/README.md) to setup a x86_64 based demo environment on AKS.

2. To prevent our changes to be rolled back, disable the built-in AKS azurefile driver:
```bash
az aks update -g ${AZURE_RESOURCE_GROUP} --name ${CLUSTER_NAME} --disable-file-driver
```

3. Assign the `Storage Account Contributor` role to the AKS agent pool application so it can create storage accounts:

```bash
OBJECT_ID="$(az ad sp list --display-name "${CLUSTER_NAME}-agentpool" --query '[].id' --output tsv)"
az role assignment create \
  --role "Storage Account Contributor" \
  --assignee-object-id ${OBJECT_ID} \
  --assignee-principal-type ServicePrincipal \
  --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}-aks"
```

### Deploy azurefile-csi-driver on the cluster
Note: All the steps can be performed anywhere with cluster access

1. Clone the azurefile-csi-driver source:
```bash
git clone --depth 1 --branch v1.28.0 https://github.com/kubernetes-sigs/azurefile-csi-driver
cd azurefile-csi-driver
```

2. Enable `attachRequired` in the CSI Driver:
```bash
sed -i 's/attachRequired: false/attachRequired: true/g' deploy/csi-azurefile-driver.yaml
```

3. Run the script:
```bash
bash ./deploy/install-driver.sh master local
```

### Build custom csi-wrapper images (for development)
Follow this if you have made changes to the CSI wrapper code and want to deploy those changes.

1. Go back to the cloud-api-adaptor directory:
```bash
cd ~/cloud-api-adaptor
```

2. Build csi-wrapper images:
```bash
cd volumes/csi-wrapper/
make csi-controller-wrapper-docker
make csi-node-wrapper-docker
make csi-podvm-wrapper-docker
cd -
```

3. Export custom registry

```bash
export REGISTRY="my-registry" # e.g. "quay.io/my-registry"
```

4. Tag and push images
```bash
docker tag docker.io/library/csi-controller-wrapper:local ${REGISTRY}/csi-controller-wrapper:latest
docker tag docker.io/library/csi-node-wrapper:local ${REGISTRY}/csi-node-wrapper:latest
docker tag docker.io/library/csi-podvm-wrapper:local ${REGISTRY}/csi-podvm-wrapper:latest

docker push ${REGISTRY}/csi-controller-wrapper:latest
docker push ${REGISTRY}/csi-node-wrapper:latest
docker push ${REGISTRY}/csi-podvm-wrapper:latest
```

5. Change image in CSI wrapper k8s resources
```bash
sed -i "s#quay.io/confidential-containers#${REGISTRY}#g" volumes/csi-wrapper/examples/azure/*.yaml
```

### Deploy csi-wrapper to patch azurefiles-csi-driver

1. Go back to the cloud-api-adaptor directory
```bash
cd ~/cloud-api-adaptor
```

2. Create the PeerpodVolume CRD object
```bash
kubectl apply -f volumes/csi-wrapper/crd/peerpodvolume.yaml
```

The output looks like:
```bash
customresourcedefinition.apiextensions.k8s.io/peerpodvolumes.confidentialcontainers.org created
```

3. Configure RBAC so that the wrapper has access to the required operations
```bash
kubectl apply -f volumes/csi-wrapper/examples/azure/azure-files-csi-wrapper-runner.yaml
kubectl apply -f volumes/csi-wrapper/examples/azure/azure-files-csi-wrapper-podvm.yaml
```

4. patch csi-azurefile-driver:
```bash
kubectl patch deploy csi-azurefile-controller -n kube-system --patch-file volumes/csi-wrapper/examples/azure/patch-controller.yaml
kubectl -n kube-system delete replicaset -l app=csi-azurefile-controller
kubectl patch ds csi-azurefile-node -n kube-system --patch-file volumes/csi-wrapper/examples/azure/patch-node.yaml
```

5. Create **storage class**:
```bash
kubectl apply -f volumes/csi-wrapper/examples/azure/azure-file-StorageClass-for-peerpod.yaml
```

### Run the `csi-wrapper for peerpod storage` demo

1. Create one pvc that use `azurefiles-csi-driver`
```bash
kubectl apply -f volumes/csi-wrapper/examples/azure/my-pvc.yaml
```

2. Wait for the pvc status to become `bound`
```bash
$ k get pvc
NAME            STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS         AGE
pvc-azurefile   Bound    pvc-3edc7a93-4531-4034-8818-1b1608907494   1Gi        RWO            azure-file-storage   3m11s
```

3. Create the nginx peer-pod demo with with `podvm-wrapper` and `azurefile-csi-driver` containers
```bash
kubectl apply -f volumes/csi-wrapper/examples/azure/nginx-kata-with-my-pvc-and-csi-wrapper.yaml
```

4. Exec into the container and check the mount

```bash
kubectl exec nginx-pv -c nginx -i -t -- sh
# mount | grep mount-path
//fffffffffffffffffffffff.file.core.windows.net/pvc-ff587660-73ed-4bd0-8850-285be480f490 on /mount-path type cifs (rw,relatime,vers=3.1.1,cache=strict,username=fffffffffffffffffffffff,uid=0,noforceuid,gid=0,noforcegid,addr=x.x.x.x,file_mode=0777,dir_mode=0777,soft,persistenthandles,nounix,serverino,mapposix,mfsymlinks,rsize=1048576,wsize=1048576,bsize=1048576,echo_interval=60,actimeo=30,closetimeo=1)
```

**Note:** We can see there's a CIFS mount to `/mount-path` as expected
