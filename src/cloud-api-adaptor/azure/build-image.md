# Pod-VM image for Azure

This documentation will walk you through building the pod VM image for Azure.

## Pre-requisites

### Install required tools

This assumes an Ubuntu 24.04 environment for building. When using a different distribution, adjust accordingly:

```bash
sudo apt-get update -y
sudo apt-get install -y \
  alien \
  bubblewrap \
  build-essential \
  dnf \
  mtools \
  qemu-utils \
  systemd-ukify \
  uidmap
sudo snap install yq
```

### Install mkosi

Clone mkosi and add the bin folder to the PATH:

```bash
MKOSI_VERSION=$(yq -e '.tools.mkosi' versions.yaml)"
git clone -b "$MKOSI_VERSION" https://github.com/systemd/mkosi
export PATH="$PWD/mkosi/bin:$PATH"
mkosi --version
```

### Install uplosi

The tool is required to publish images to an image gallery.

```bash
wget "https://github.com/edgelesssys/uplosi/releases/download/v0.3.0/uplosi_0.3.0_linux_amd64.tar.gz"
tar xzf uplosi_0.3.0_linux_amd64.tar.gz uplosi
sudo mv uplosi mkosi/bin/
```

### Azure login

The image build will use your local credentials, so make sure you have logged into your account via `az login`.

```bash
export AZURE_SUBSCRIPTION_ID=$(az account show --query id --output tsv)
export AZURE_REGION="eastus"
```

## Build

### Binaries

Build binaries with support for the Azure CVM TEEs and verify the provenance attestation for the binaries consumed from upstream:

```bash
cd ../podvm-mkosi
TEE_PLATFORM=az-cvm-vtpm VERIFY_PROVENANCE=yes make binaries
```

### Image

You can build a debug image in which you can login as root by providing an SSH key as a build asset, the debug-image will also auto-login on the serial console, and contain some extraneous packages for debugging:

```bash
cp ~/.ssh/id_rsa.pub podvm-mkosi/resources/authorized_keys
make image-debug
```

Otherwise, you can build a release image:

```bash
make image
``` 

## Publish

We can upload the built image to an Azure Image Gallery. The resulting image id can be used as `AZURE_IMAGE_ID` param in the [kustomization.yaml](../install/overlays/azure/kustomization.yaml) file.

```bash
SHARING_NAME_PREFIX=sharedcocopodvms # set for a community gallery
IMAGE_VERSION=0.1.0
IMAGE_DEFINITION=podvm
IMAGE_GALLERY=coco
RESOURCE_GROUP=myrg
SUBSCRIPTION_ID=mysub

cat <<EOF> uplosi.conf
[base]
imageVersion = \"$IMAGE_VERSION\"
name = \"$IMAGE_DEFINITION\"

[base.azure]
subscriptionID = \"$SUBSCRIPTION_ID\"
location = "eastus"
resourceGroup = \"$RESOURCE_GROUP\"
sharedImageGallery = \"$IMAGE_GALLERY\"
${SHARING_NAME_PREFIX:+sharingNamePrefix = \"$SHARING_NAME_PREFIX\"}

[variant.default]
provider = "azure"

[variant.default.azure]
replicationRegions = ["eastus","eastus2","westeurope","northeurope"]
EOF

uplosi upload build/system.raw
```
