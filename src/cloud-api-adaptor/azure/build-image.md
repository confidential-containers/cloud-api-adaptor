# Pod-VM image for Azure

This documentation will walk you through building the pod VM image for Azure.

> [!NOTE]
> Run the following commands from the directory `azure/image`.

## Pre-requisites

### Install required tools

- Install tools like `git`, `make` and `curl`.
- Install Azure CLI by following instructions [here](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli).

### Azure login

The image build will use your local credentials, so make sure you have logged into your account via `az login`. Retrieve your Subscription ID and set your preferred region:

```bash
export AZURE_SUBSCRIPTION_ID=$(az account show --query id --output tsv)
export AZURE_REGION="eastus"
```

### Resource group

> [!NOTE]
> Skip this step if you already have a resource group you want to use. Please, export the resource group name in the `AZURE_RESOURCE_GROUP` environment variable.

Create an Azure resource group by running the following command:

```bash
export AZURE_RESOURCE_GROUP="caa-rg-$(date '+%Y%m%b%d%H%M%S')"

az group create \
  --name "${AZURE_RESOURCE_GROUP}" \
  --location "${AZURE_REGION}"
```

### Shared image gallery

Create a shared image gallery:

```bash
export GALLERY_NAME="caaubntcvmsGallery"
az sig create \
  --gallery-name "${GALLERY_NAME}" \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --location "${AZURE_REGION}"
```

Create the "Image Definition" by running the following command:

> [!NOTE]
> The flag `--features SecurityType=ConfidentialVmSupported` allows you to a upload custom image and boot it up as a Confidential Virtual Machine (CVM).

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

## Build pod-VM image

The Pod-VM image can be built in three ways:

- Customize an existing marketplace image
- Customize an existing marketplace image with pre-built binaries
- Convert and upload a pre-built QCOW2 image

### Option 1: Modifying an existing marketplace image

**Install necessary tools**

- Install the following packages (on Ubuntu):

```bash
sudo apt install \
  libdevmapper-dev libgpgme-dev gcc clang pkg-config \
  libssl-dev libtss2-dev protobuf-compiler
```

- Install `yq` by following instructions [here](https://mikefarah.gitbook.io/yq/#install).
- Install Golang by following instructions [here](https://go.dev/doc/install).
- Install packer by following [these instructions](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).

**Build**

- Create a custom Azure VM image based on Ubuntu 22.04 adding kata-agent, agent-protocol-forwarder and other dependencies for Cloud API Adaptor (CAA):

```bash
export PKR_VAR_resource_group="${AZURE_RESOURCE_GROUP}"
export PKR_VAR_location="${AZURE_REGION}"
export PKR_VAR_subscription_id="${AZURE_SUBSCRIPTION_ID}"
export PKR_VAR_use_azure_cli_auth=true
export PKR_VAR_az_gallery_name="${GALLERY_NAME}"
export PKR_VAR_az_gallery_image_name="${GALLERY_IMAGE_DEF_NAME}"
export PKR_VAR_az_gallery_image_version="0.0.1"
export PKR_VAR_offer=0001-com-ubuntu-confidential-vm-jammy
export PKR_VAR_sku=22_04-lts-cvm

export TEE_PLATFORM="az-cvm-vtpm"
export LIBC=gnu
export CLOUD_PROVIDER=azure
PODVM_DISTRO=ubuntu make image
```

> [!NOTE]
> If you want to disable cloud-init then `export DISABLE_CLOUD_CONFIG=true` before building the image.

Use the `ManagedImageSharedImageGalleryId` field from output of the above command to populate the following environment variable. It's used while deploying cloud-api-adaptor:

```bash
# e.g. format: /subscriptions/.../resourceGroups/.../providers/Microsoft.Compute/galleries/.../images/.../versions/../
export AZURE_IMAGE_ID="/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}/providers/Microsoft.Compute/galleries/${GALLERY_NAME}/images/${GALLERY_IMAGE_DEF_NAME}/versions/${PKR_VAR_az_gallery_image_version}"
```

### Option 2: Customize an image using prebuilt binaries via Docker

```bash
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

If you want to use a different base image, then you'll need to export environment variables: `PUBLISHER`, `OFFER` and `SKU`.

Sometimes using the marketplace image requires accepting a licensing agreement and also using a published plan.
Following [link](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/cli-ps-findimage) provides more detail.

For example using the CentOS 8.5 image from "eurolinux" publisher requires a plan and license agreement.

You'll need to first get the Uniform Resource Name (URN):

```bash
az vm image list \
  --location ${AZURE_REGION} \
  --publisher eurolinuxspzoo1620639373013 \
  --offer centos-8-5-free \
  --sku centos-8-5-free \
  --all \
  --output table
```

Then you'll need to accept the agreement:

```bash
az vm image terms accept \
    --urn eurolinuxspzoo1620639373013:centos-8-5-free:centos-8-5-free:8.5.5
```

Then you can use the following command line to build the image:

```bash
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

Another example of building Red Hat Enterprise Linux (RHEL) based image:

```bash
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

### Option 3: Using a pre-created QCOW2 image

`quay.io/confidential-containers` hosts pre-created pod-vm images as container images.

- Download QCOW2 image

```bash
mkdir -p qcow2-img && cd qcow2-img

export QCOW2_IMAGE="quay.io/confidential-containers/podvm-generic-ubuntu-amd64:latest"
curl -LO https://raw.githubusercontent.com/confidential-containers/cloud-api-adaptor/staging/podvm/hack/download-image.sh

bash download-image.sh $QCOW2_IMAGE . -o podvm.qcow2
```

- Convert QCOW2 image to Virtual Hard Disk (VHD) format

You'll need the `qemu-img` tool for conversion.

```bash
qemu-img convert -O vpc -o subformat=fixed,force_size podvm.qcow2 podvm.vhd
```

- Create Storage Account

Create a storage account if none exists. Otherwise you can use the existing storage account.

```bash
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

```bash
export AZURE_STORAGE_CONTAINER=vhd
az storage container create \
    --account-name $AZURE_STORAGE_ACCOUNT \
    --name $AZURE_STORAGE_CONTAINER \
    --auth-mode login
```

- Get storage key

```bash
AZURE_STORAGE_KEY=$(az storage account keys list --resource-group $AZURE_RESOURCE_GROUP --account-name $AZURE_STORAGE_ACCOUNT --query "[?keyName=='key1'].{Value:value}" --output tsv)

echo $AZURE_STORAGE_KEY
```

- Upload VHD file to Azure Storage

```bash
az storage blob upload  --container-name $AZURE_STORAGE_CONTAINER --name podvm.vhd --file podvm.vhd
```

- Get the VHD URI

```bash
AZURE_STORAGE_EP=$(az storage account list -g $AZURE_RESOURCE_GROUP --query "[].{uri:primaryEndpoints.blob} | [? contains(uri, '$AZURE_STORAGE_ACCOUNT')]" --output tsv)

echo $AZURE_STORAGE_EP

export VHD_URI="${AZURE_STORAGE_EP}${AZURE_STORAGE_CONTAINER}/podvm.vhd"
```

- Create Azure VM Image Version

```bash
az sig image-version create \
   --resource-group $AZURE_RESOURCE_GROUP \
   --gallery-name $GALLERY_NAME  \
   --gallery-image-definition $GALLERY_IMAGE_DEF_NAME \
   --gallery-image-version 0.0.1 \
   --target-regions $AZURE_REGION \
   --os-vhd-uri "$VHD_URI" \
   --os-vhd-storage-account $AZURE_STORAGE_ACCOUNT
```

On success, the command will generate the image id. Set this image id as a value of `AZURE_IMAGE_ID` in `peer-pods-cm` Configmap.

You can also use the following command to retrieve the image id:

```bash
AZURE_IMAGE_ID=$(az sig image-version  list --resource-group  $AZURE_RESOURCE_GROUP --gallery-name $GALLERY_NAME --gallery-image-definition $GALLERY_IMAGE_DEF_NAME --query "[].{Id: id}" --output tsv)

echo $AZURE_IMAGE_ID
```
