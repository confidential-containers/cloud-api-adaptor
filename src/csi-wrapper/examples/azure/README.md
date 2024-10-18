# Azure CSI Wrapper for Peer Pod Storage

## Set up a demo environment on your development machine

1. Follow the [README.md](../../../cloud-api-adaptor/azure/README.md) to setup a x86_64 based demo environment on AKS.

2. To prevent our changes to be rolled back, disable the built-in AKS azurefile and azuredisk drivers:

    ```bash
    az aks update -g ${AZURE_RESOURCE_GROUP} --name ${CLUSTER_NAME} --disable-file-driver --disable-disk-driver
    ```

3. Create the PeerpodVolume CRD object

    ```bash
    kubectl apply -f src/csi-wrapper/crd/peerpodvolume.yaml
    ```

    The output looks like:

    ```bash
    customresourcedefinition.apiextensions.k8s.io/peerpodvolumes.confidentialcontainers.org created
    ```

## Build custom csi-wrapper images (for development)

Follow this if you have made changes to the CSI wrapper code and want to deploy those changes.

1. Build csi-wrapper images:

    ```bash
    pushd src/csi-wrapper/
    make csi-controller-wrapper-docker
    make csi-node-wrapper-docker
    make csi-podvm-wrapper-docker
    popd
    ```

3. Export custom registry

    ```bash
    export REGISTRY="my-registry" # e.g. "quay.io/my-registry"
    ```

4. Tag and push images

    ```bash
    docker tag csi-controller-wrapper:local ${REGISTRY}/csi-controller-wrapper:latest
    docker tag csi-node-wrapper:local ${REGISTRY}/csi-node-wrapper:latest
    docker tag csi-podvm-wrapper:local ${REGISTRY}/csi-podvm-wrapper:latest

    docker push ${REGISTRY}/csi-controller-wrapper:latest
    docker push ${REGISTRY}/csi-node-wrapper:latest
    docker push ${REGISTRY}/csi-podvm-wrapper:latest
    ```

5. Change image in CSI wrapper k8s resources

    ```bash
    sed -i "s#quay.io/confidential-containers#${REGISTRY}#g" volumes/csi-wrapper/examples/azure/disk/*.yaml
    sed -i "s#quay.io/confidential-containers#${REGISTRY}#g" volumes/csi-wrapper/examples/azure/file/*.yaml
    ```

## Peer Pod example using CSI Wrapper with azurefile-csi-driver

Prerequisite: Assign the `Storage Account Contributor` role to the AKS agent pool application so it can create storage accounts:

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
    pushd azurefile-csi-driver
    ```

2. Enable `attachRequired` in the CSI Driver:

    ```bash
    sed -i 's/attachRequired: false/attachRequired: true/g' deploy/csi-azurefile-driver.yaml
    ```

3. Run the script:

    ```bash
    bash ./deploy/install-driver.sh master local
    popd
    ```

### Deploy csi-wrapper to patch azurefile-csi-driver

1. Configure RBAC so that the wrapper has access to the required operations

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/file/azure-files-csi-wrapper-runner.yaml
    kubectl apply -f src/csi-wrapper/examples/azure/file/azure-files-csi-wrapper-podvm.yaml
    ```

2. Patch csi-azurefile-driver:

    ```bash
    kubectl patch deploy csi-azurefile-controller -n kube-system --patch-file src/csi-wrapper/examples/azure/file/patch-controller.yaml
    kubectl -n kube-system delete replicaset -l app=csi-azurefile-controller
    kubectl patch ds csi-azurefile-node -n kube-system --patch-file src/csi-wrapper/examples/azure/file/patch-node.yaml
    ```

3. Create a peerpod enabled StorageClass:

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/file/azure-file-StorageClass-for-peerpod.yaml
    ```

### Create a Pod with a PVC using the azurefile-csi-driver

1. Create a PVC that use `azurefile-csi-driver`

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/file/my-pvc.yaml
    ```

2. Wait for the PVC status to become `bound`

    ```bash
    $ kubectl get pvc
    NAME            STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS         AGE
    pvc-azurefile   Bound    pvc-3edc7a93-4531-4034-8818-1b1608907494   1Gi        RWO            azure-file-storage   3m11s
    ```

3. Create the nginx peer-pod demo with with `podvm-wrapper` and `azurefile-csi-driver` containers

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/file/nginx-kata-with-my-pvc-and-csi-wrapper.yaml
    ```

4. Exec into the container and check the mount

    ```bash
    kubectl exec nginx-pv -c nginx -i -t -- sh
    # mount | grep mount-path
    //fffffffffffffffffffffff.file.core.windows.net/pvc-ff587660-73ed-4bd0-8850-285be480f490 on /mount-path type cifs (rw,relatime,vers=3.1.1,cache=strict,username=fffffffffffffffffffffff,uid=0,noforceuid,gid=0,noforcegid,addr=x.x.x.x,file_mode=0777,dir_mode=0777,soft,persistenthandles,nounix,serverino,mapposix,mfsymlinks,rsize=1048576,wsize=1048576,bsize=1048576,echo_interval=60,actimeo=30,closetimeo=1)
    ```

    **Note:** We can see there's a CIFS mount to `/mount-path` as expected

## Peer Pod example using CSI Wrapper with azuredisk-csi-driver

Prerequisite: Assign the `Contributor` role to the AKS agent pool application so it can create storage accounts:

```bash
OBJECT_ID="$(az ad sp list --display-name "${CLUSTER_NAME}-agentpool" --query '[].id' --output tsv)"
az role assignment create \
  --role "Contributor" \
  --assignee-object-id ${OBJECT_ID} \
  --assignee-principal-type ServicePrincipal \
  --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}-aks"
```

### Deploy azuredisk-csi-driver on the cluster

1. Clone the azuredisk-csi-driver source:

    ```bash
    git clone --depth 1 --branch v1.30.3 https://github.com/kubernetes-sigs/azuredisk-csi-driver
    ```

2. Run the script:

    ```bash
    pushd azuredisk-csi-driver
    bash ./deploy/install-driver.sh master local
    popd
    ```

### Deploy csi-wrapper to patch azuredisk-csi-driver

1. Configure RBAC so that the wrapper has access to the required operations

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/disk/azure-disk-csi-wrapper-runner.yaml
    kubectl apply -f src/csi-wrapper/examples/azure/disk/azure-disk-csi-wrapper-podvm.yaml
    ```

2. Patch csi-azuredisk-driver:

    ```bash
    kubectl patch deploy csi-azuredisk-controller -n kube-system --patch-file src/csi-wrapper/examples/azure/disk/patch-controller.yaml
    kubectl -n kube-system delete replicaset -l app=csi-azuredisk-controller
    kubectl patch ds csi-azuredisk-node -n kube-system --patch-file src/csi-wrapper/examples/azure/disk/patch-node.yaml
    ```

3. Create a peerpod enabled StorageClass:

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/disk/azure-disk-storageclass-for-peerpod.yaml
    ```

#### Option A: Create a Pod with a dynamically provisioned PVC

1. Create a PVC that use `azuredisk-csi-driver`

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/disk/dynamic-pvc.yaml
    ```

2. Wait for the PVC status to become `bound`

    ```bash
    $ kubectl get pvc
    NAME            STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS         AGE
    pvc-azuredisk   Bound    pvc-3edc7a93-4531-4034-8818-1b1608907494   10Gi       RWO            azure-disk-storage   3m11s
    ```

#### Option B: Create a Pod with a statically provisioned PVC using an existing disk

1. Create a disk in Azure

    ```bash
    az disk create --resource-group "${AZURE_RESOURCE_GROUP}-aks" --name static-pvc --size-gb 10 --sku Standard_LRS
    azure_disk_id=$(az disk show --resource-group "${AZURE_RESOURCE_GROUP}-aks" --name static-pvc --query id --output tsv)
    ```

2. Update the PVC yaml file to use the newly created disk as backend

    ```bash
    sed -i "s|@@AZURE_DISK_ID@@|$azure_disk_id|g" src/csi-wrapper/examples/azure/disk/static-pvc.yaml
    ```

3. Create a PVC that use `azuredisk-csi-driver` and the statically provisioned disk

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/disk/static-pvc.yaml
    ```

4. Wait for the PVC status to become `bound`

    ```bash
    $ kubectl get pvc
    NAME            STATUS   VOLUME         CAPACITY   ACCESS MODES   STORAGECLASS         VOLUMEATTRIBUTESCLASS   AGE
    pvc-azuredisk   Bound    pv-azuredisk   10Gi       RWO            azure-disk-storage   <unset>                 36s
    ```

### Create a Pod with a PVC using the azuredisk-csi-driver

1. Create the nginx peer-pod demo with with `podvm-wrapper` and `azuredisk-csi-driver` containers

    ```bash
    kubectl apply -f src/csi-wrapper/examples/azure/disk/nginx-kata-with-my-pvc-and-csi-wrapper.yaml
    ```

2. Exec into the container and check the mount

    ```bash
    kubectl exec nginx-pv-disk -c nginx -i -t -- sh
    # mount | grep mount-path
    /dev/sdb on /mount-path type ext4 (rw,relatime)
    ```

    **Note:** We can see there's a ext4 mount to `/mount-path` as expected
