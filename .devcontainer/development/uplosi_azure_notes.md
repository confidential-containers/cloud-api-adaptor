## Edit 
```console
/workspaces/cloud-api-adaptor/src/cloud-api-adaptor/versions.yaml
```
oci.guest-components.reference to desired tag found here (non sha) : [ghcr.io/confidential-containers/guest-components](https://github.com/orgs/confidential-containers/packages?repo_name=guest-components)


## mkosi build debug podvm

```console
cd /workspaces/cloud-api-adaptor/src/cloud-api-adaptor/podvm-mkosi
TEE_PLATFORM=az-cvm-vtpm make debug
```

## Set envs

```console
export AZURE_COMMUNITY_GALLERY_NAME=cocopodvm
export AZURE_PODVM_GALLERY_NAME=imgcocoimages

export AZURE_PODVM_IMAGE_DEF_NAME= # podvm_image0_debug or podvm_image0
export AZURE_PODVM_IMAGE_VERSION=

export AZURE_SUBSCRIPTION_ID=
export AZURE_RESOURCE_GROUP=
```

## Add azure cli

```console
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
az login
```

## Create uplosi config


```console
SHARING_NAME_PREFIX="$(echo $AZURE_COMMUNITY_GALLERY_NAME | cut -d'-' -f1)"
cat <<EOF> uplosi.conf
[base]
imageVersion = "$AZURE_PODVM_IMAGE_VERSION"
name = "$AZURE_PODVM_IMAGE_DEF_NAME"

[variant.default]
provider = "azure"


[base.azure]
subscriptionID = "$AZURE_SUBSCRIPTION_ID"
location = "westeurope"
resourceGroup = "$AZURE_RESOURCE_GROUP"
sharedImageGallery = "$AZURE_PODVM_GALLERY_NAME"
sharingNamePrefix = "$SHARING_NAME_PREFIX"


EOF
```


## Run uplosi

```console
$HOME/go/bin/uplosi upload build/system.raw
```
