# Cloud API Adaptor on Azure

This documentation will walk you through setting up Cloud API Adaptor (CAA) on
Azure Kubernetes Service (AKS). We will build the pod vm image, CAA's
application image, deploy one worker AKS, deploy CAA on that Kubernetes cluster
and finally deploy a sample application that will run as a pod backed by CAA
pod VM.

## Pre-requisites

### Azure Login

The image build will use your local credentials, so make sure you have
logged into your account via `az login`. Retrieve your Subscription ID
and set your preferred region:

```bash
export AZURE_SUBSCRIPTION_ID=$(az account show --query id --output tsv)
export AZURE_REGION="REPLACE_ME"
```

### Resource Group

We will use this resource group for all of our deployments.
Create an Azure resource group by running the following command:

```bash
export AZURE_RESOURCE_GROUP="REPLACE_ME"
az group create --name "${AZURE_RESOURCE_GROUP}" --location "${AZURE_REGION}"
```

### Shared Image Gallery

Create a shared image gallery:

```bash
export GALLERY_NAME="caaubntcvmsGallery"
az sig create \
  --gallery-name "${GALLERY_NAME}" \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --location "${AZURE_REGION}"
```

Create the Image Definition by running the following command. Do
note that the flag `--features SecurityType=ConfidentialVmSupported`
allows us to a upload custom image and boot it up as a CVM.

```bash
export GALLERY_IMAGE_DEF_NAME="cc-image"
az sig image-definition create \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --gallery-name "${GALLERY_NAME}" \
  --gallery-image-definition "${GALLERY_IMAGE_DEF_NAME}" \
  --publisher GreatPublisher \
  --offer GreatOffer \
  --sku GreatSku \
  --os-type "Linux" \
  --os-state "Generalized" \
  --hyper-v-generation "V2" \
  --location "${AZURE_REGION}" \
  --architecture "x64" \
  --features SecurityType=ConfidentialVmSupported
```

## Build Pod VM Image

There are three options:

- Customize an existing marketplace image
- Customize an existing marketplace image with pre-built binaries
- Convert and upload a pre-built QCOW2 image

### Option 1: Modifying an Existing Marketplace Image

- Install packer by following [these instructions](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).

- Create a custom Azure VM image based on Ubuntu 22.04 adding kata-agent, agent-protocol-forwarder and other dependencies for CAA:

```bash
cd image
export PKR_VAR_resource_group="${AZURE_RESOURCE_GROUP}"
export PKR_VAR_location="${AZURE_REGION}"
export PKR_VAR_subscription_id="${AZURE_SUBSCRIPTION_ID}"
export PKR_VAR_use_azure_cli_auth=true
export PKR_VAR_az_gallery_name="${GALLERY_NAME}"
export PKR_VAR_az_gallery_image_name="${GALLERY_IMAGE_DEF_NAME}"
export PKR_VAR_offer=0001-com-ubuntu-confidential-vm-jammy
export PKR_VAR_sku=22_04-lts-cvm
export CLOUD_PROVIDER=azure
PODVM_DISTRO=ubuntu make image && cd -
```


> **NOTE**: If you want to disable cloud config then `export DISABLE_CLOUD_CONFIG=true` before building the image.

Use the `ManagedImageSharedImageGalleryId` field from output of the above command to populate the following environment variable it will be used while deploying the cloud-api-adaptor:

```bash
# e.g. format: /subscriptions/.../resourceGroups/.../providers/Microsoft.Compute/galleries/.../images/.../versions/..
export AZURE_IMAGE_ID="REPLACE_ME"
```

### Option 2: Customize an Image Using Prebuilt Binaries via Docker

```bash
cd image
docker build -t azure-podvm-builder .
```

```bash
docker run --rm \
  -v "$HOME/.azure:/root/.azure" \
  -e AZURE_SUBSCRIPTION_ID \
  -e AZURE_RESOURCE_GROUP \
  -e GALLERY_NAME \
  -e GALLERY_IMAGE_DEF_NAME \
  azure-podvm-builder
```

If you want to use a different base image, then you'll need to provide additional envs: `PUBLISHER`, `OFFER`, `SKU`

Sometimes using the marketplace image requires accepting a licensing agreement and also using a published plan.
Following [link](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/cli-ps-findimage) provides more detail.

For example using the CentOS 8.5 image from eurolinux publisher requires a plan and license agreement.
You'll need to first get the URN:
```
az vm image list \
  --location ${AZURE_REGION} \
  --publisher eurolinuxspzoo1620639373013 \
  --offer centos-8-5-free \
  --sku centos-8-5-free \
  --all \
  --output table
```
Then you'll need to accept the agreement:
```
az vm image terms accept --urn eurolinuxspzoo1620639373013:centos-8-5-free:centos-8-5-free:8.5.5
```

Then you can use the following command line to build the image:
```
docker run --rm \
  -v "$HOME/.azure:/root/.azure" \
  -e AZURE_SUBSCRIPTION_ID \
  -e AZURE_RESOURCE_GROUP  \
  -e PUBLISHER=eurolinuxspzoo1620639373013 \
  -e SKU=centos-8-5-free \
  -e OFFER=centos-8-5-free \
  -e PLAN_NAME=centos-8-5-free \
  -e PLAN_PRODUCT=centos-8-5-free \
  -e PLAN_PUBLISHER=eurolinuxspzoo1620639373013 \
  -e PODVM_DISTRO=centos \
  azure-podvm-builder
```

Here is another example of building RHEL based image:

```
docker run --rm \
  -v "$HOME/.azure:/root/.azure" \
  -e AZURE_SUBSCRIPTION_ID \
  -e AZURE_RESOURCE_GROUP  \
  -e PUBLISHER=RedHat \
  -e SKU=9-lvm \
  -e OFFER=RHEL \
  -e PODVM_DISTRO=rhel \
  azure-podvm-builder
```

### Option 3: Using a precreated QCOW2 image

The precreated images are available as container images from `quay.io/confidential-containers`

- Download QCOW2 image
```
mkdir -p qcow2-img && cd qcow2-img

export QCOW2_IMAGE="quay.io/confidential-containers/podvm-generic-ubuntu-amd64:latest"
curl -LO https://raw.githubusercontent.com/confidential-containers/cloud-api-adaptor/staging/podvm/hack/download-image.sh

bash download-image.sh $QCOW2_IMAGE . -o podvm.qcow2

```

- Convert QCOW2 image to VHD format
You'll need the `qemu-img` tool for conversion.
```
qemu-img convert -O vpc -o subformat=fixed,force_size podvm.qcow2 podvm.vhd
```

- Create Storage Account
Create a storage account if none exists. Otherwise you can use the existing storage account.
```
export AZURE_STORAGE_ACCOUNT=cocosa

az storage account create \
--name $AZURE_STORAGE_ACCOUNT  \
    --resource-group $AZURE_RESOURCE_GROUP \
    --location $AZURE_REGION \
    --sku Standard_ZRS \
    --encryption-services blob
```

- Create storage container
Create a storage container if none exists. Otherwise you can use the existing storage account

```
export AZURE_STORAGE_CONTAINER=vhd
az storage container create \
    --account-name $AZURE_STORAGE_ACCOUNT \
    --name $AZURE_STORAGE_CONTAINER \
    --auth-mode login
```

- Get storage key
```
AZURE_STORAGE_KEY=$(az storage account keys list --resource-group $AZURE_RESOURCE_GROUP --account-name $AZURE_STORAGE_ACCOUNT --query "[?keyName=='key1'].{Value:value}" --output tsv)

echo $AZURE_STORAGE_KEY
```

- Upload VHD file to Azure Storage
```
az storage blob upload  --container-name $AZURE_STORAGE_CONTAINER --name podvm.vhd --file podvm.vhd
```

- Get the VHD URI
```
AZURE_STORAGE_EP=$(az storage account list -g $AZURE_RESOURCE_GROUP --query "[].{uri:primaryEndpoints.blob} | [? contains(uri, '$AZURE_STORAGE_ACCOUNT')]" --output tsv)

echo $AZURE_STORAGE_EP

export VHD_URI="${AZURE_STORAGE_EP}${AZURE_STORAGE_CONTAINER}/podvm.vhd"
```

- Create Azure VM Image Version
```
az sig image-version create \
   --resource-group $AZURE_RESOURCE_GROUP \
   --gallery-name $GALLERY_NAME  \
   --gallery-image-definition $GALLERY_IMAGE_DEF_NAME \
   --gallery-image-version 0.0.1 \
   --target-regions $AZURE_REGION \
   --os-vhd-uri "$VHD_URI" \
   --os-vhd-storage-account $AZURE_STORAGE_ACCOUNT
```

On success, the command will generate the image id, which needs to be used to set the value of `AZURE_IMAGE_ID` in `peer-pods-cm` configmap.
You can also use the following command to retrieve the image id
```
AZURE_IMAGE_ID=$(az sig image-version  list --resource-group  $AZURE_RESOURCE_GROUP --gallery-name $GALLERY_NAME --gallery-image-definition $GALLERY_IMAGE_DEF_NAME --query "[].{Id: id}" --output tsv)

echo $AZURE_IMAGE_ID
```

## Build CAA Container Image

If you have made changes to the Cloud API Adaptor code and if you want to deploy those changes then follow [these instructions](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/install/README.md#building-custom-cloud-api-adaptor-image) to build the container image from the root of this repository.

If you would like to deploy the latest code from the default branch (`main`) of this repository then just expose the following environment variable:

```bash
export registry="quay.io/confidential-containers"
```

## Deploy Kubernetes using AKS

Make changes to the following environment variable as you see fit:

```bash
export CLUSTER_NAME="REPLACE_ME"
export AKS_WORKER_USER_NAME="azuser"
export SSH_KEY=~/.ssh/id_rsa.pub
export AKS_RG="${AZURE_RESOURCE_GROUP}-aks"
```

**Optional:** deploy the worker nodes into an existing VNet and Subnet:

```bash
export SUBNET_ID="REPLACE_ME"
```

Deploy AKS with single worker node to the same resource group we created earlier:

```bash
az aks create \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --node-resource-group "${AKS_RG}" \
  "${SUBNET_ID+--vnet-subnet-id $SUBNET_ID}" \
  --name "${CLUSTER_NAME}" \
  --enable-oidc-issuer \
  --enable-workload-identity \
  --location "${AZURE_REGION}" \
  --node-count 1 \
  --node-vm-size Standard_F4s_v2 \
  --ssh-key-value "${SSH_KEY}" \
  --admin-username "${AKS_WORKER_USER_NAME}" \
  --os-sku Ubuntu
```

Download kubeconfig locally to access the cluster using `kubectl`:

```bash
az aks get-credentials \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --name "${CLUSTER_NAME}"
```

Label the nodes so that CAA can be deployed on it:

```bash
kubectl label nodes --all node.kubernetes.io/worker=
```

## Deploy Cloud API Adaptor

> **NOTE**: If you are using Calico CNI on a different Kubernetes cluster,
> then,
> [configure](https://projectcalico.docs.tigera.io/networking/vxlan-ipip#configure-vxlan-encapsulation-for-all-inter-workload-traffic)
> VXLAN encapsulation for all inter workload traffic.

### User Assigned Identity and Federated Credentials

We will use a Workload Identity to provide the CAA DaemonSet with the
privileges it requires to manage virtual machines in our resource group.
Note: if you use an existing AKS cluster it might need to be configured
to support Workload Identity and OIDC, please refer to the instructions
in [this guide](https://learn.microsoft.com/en-us/azure/aks/workload-identity-deploy-cluster#update-an-existing-aks-cluster).

```bash
az identity create --name caa-identity -g "$AZURE_RESOURCE_GROUP" -l "$AZURE_REGION"
export USER_ASSIGNED_CLIENT_ID="$(az identity show --resource-group "$AZURE_RESOURCE_GROUP" --name caa-identity --query 'clientId' -otsv)"
```

We'll annotate the CAA Service Account with the Workload Identity's
`CLIENT_ID` and make the CAA DaemonSet use Workload Identity
for authentication:

```bash
cat <<EOF > install/overlays/azure/workload-identity.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: cloud-api-adaptor-daemonset
  namespace: confidential-containers-system
spec:
  template:
    metadata:
      labels:
        azure.workload.identity/use: "true"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cloud-api-adaptor
  namespace: confidential-containers-system
  annotations:
    azure.workload.identity/client-id: "$USER_ASSIGNED_CLIENT_ID"
EOF
```

### AKS Resource Group permissions

For CAA to be able to manage VMs we need to assign the identity
VM and Network contributor roles: privileges to spawn VMs in
`$AZURE_RESOURCE_GROUP` and attach to a VNet in `$AKS_RG`.

```bash
az role assignment create --role "Virtual Machine Contributor" --assignee "$USER_ASSIGNED_CLIENT_ID" --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourcegroups/${AZURE_RESOURCE_GROUP}"
az role assignment create --role "Network Contributor" --assignee "$USER_ASSIGNED_CLIENT_ID" --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourcegroups/${AKS_RG}"
```

We then create the federated credential for the CAA ServiceAccount
using the OIDC endpoint from the AKS cluster:

```bash
export AKS_OIDC_ISSUER="$(az aks show -n "$CLUSTER_NAME" -g "$AZURE_RESOURCE_GROUP" --query "oidcIssuerProfile.issuerUrl" -otsv)"
az identity federated-credential create --name caa-fedcred \
	--identity-name caa-identity \
	--resource-group "$AZURE_RESOURCE_GROUP" \
	--issuer "${AKS_OIDC_ISSUER}" \
	--subject system:serviceaccount:confidential-containers-system:cloud-api-adaptor \
	--audience api://AzureADTokenExchange
```

### AKS Subnet ID

Fetch the VNET name of that AKS created automatically:

```bash
export AZURE_VNET_NAME=$(az network vnet list \
  --resource-group "${AKS_RG}" \
  --query "[0].name" \
  --output tsv)
```

Export the subnet ID to be used for CAA daemonset deployment:

```bash
export AZURE_SUBNET_ID=$(az network vnet subnet list \
  --resource-group "${AKS_RG}" \
  --vnet-name "${AZURE_VNET_NAME}" \
  --query "[0].id" \
  --output tsv)
```

### Populate the `kustomization.yaml` File

Replace the values as needed for the following environment variables:

```bash
# For regular VMs use something like: Standard_D2as_v5, for CVMs use something like Standard_DC2as_v5.
export AZURE_INSTANCE_SIZE="REPLACE_ME"
```

Run the following command to update the [`kustomization.yaml`](../install/overlays/azure/kustomization.yaml) file:

```bash
cat <<EOF > install/overlays/azure/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
bases:
- ../../yamls
images:
- name: cloud-api-adaptor
  newName: "${registry}/cloud-api-adaptor"
  newTag: latest
generatorOptions:
  disableNameSuffixHash: true
configMapGenerator:
- name: peer-pods-cm
  namespace: confidential-containers-system
  literals:
  - CLOUD_PROVIDER="azure"
  - AZURE_SUBSCRIPTION_ID="${AZURE_SUBSCRIPTION_ID}"
  - AZURE_REGION="${AZURE_REGION}"
  - AZURE_INSTANCE_SIZE="${AZURE_INSTANCE_SIZE}"
  - AZURE_RESOURCE_GROUP="${AZURE_RESOURCE_GROUP}"
  - AZURE_SUBNET_ID="${AZURE_SUBNET_ID}"
  - AZURE_IMAGE_ID="${AZURE_IMAGE_ID}"
secretGenerator:
- name: peer-pods-secret
  namespace: confidential-containers-system
  literals: []
- name: ssh-key-secret
  namespace: confidential-containers-system
  files:
  - id_rsa.pub
patchesStrategicMerge:
- workload-identity.yaml
EOF
```

The ssh public key should be accessible to the kustomization file:

```bash
cp $SSH_KEY install/overlays/azure/id_rsa.pub
```

### Deploy CAA on the Kubernetes cluster

Run the following command to deploy CAA:

```bash
CLOUD_PROVIDER=azure make deploy
```

Generic CAA deployment instructions are also described [here](../install/README.md).

## Run Sample Application

### Ensure Runtimeclass is present

Verify that the runtime class is created after deploying CAA:

```bash
kubectl get runtimeclass
```

Once you can find a runtimeclass named `kata-remote` then you can be sure that the deployment was successful. Successful deployment will look like this:

```console
$ kubectl get runtimeclass
NAME          HANDLER       AGE
kata-remote   kata-remote   7m18s
```

### Deploy Workload

Create an nginx deployment:

```yaml
echo '
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
spec:
  selector:
    matchLabels:
      app: nginx
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      runtimeClassName: kata-remote
      containers:
      - name: nginx
        image: bitnami/nginx:1.14
        ports:
        - containerPort: 80
        imagePullPolicy: Always
' | kubectl apply -f -
```

Ensure that the pod is up and running:
```bash
kubectl get pods -n default
```

You can verify that the pod vm was created by running the following command:
```bash
az vm list \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --output table
```

Here you should see the vm associated with the pod `nginx`. If you run into problems then check the troubleshooting guide [here](../docs/troubleshooting/README.md).

## Cleanup

If you wish to clean up the whole set up, you can delete the resource group by running the following command:
```bash
az group delete \
  --name "${AZURE_RESOURCE_GROUP}" \
  --yes --no-wait
```
