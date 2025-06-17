# az-copy-image

The tool exports an Azure Community Gallery image version into your own Shared Image Gallery. It creates a temporary managed disk and managed image from the source community image and publishes an image version under the specified gallery and image definition.

Temporary resources are removed automatically when the command completes.

The whole ceremony is required b/c as of the time of writing, the Azure API doesn't support creating cvm-enabled image-versions directly from community gallery images. In the future this tool may be superseded by the `az sig image-version create` command.

## Building

Run `make az-copy-image` from the `src/cloud-api-adaptor` directory to build the binary.

## Usage

```bash
az-copy-image \
  -resource-group <name> \
  -image-gallery <gallery> \
  -image-definition <definition> \
  -community-image-id <community image version id>
  [-subscription-id <id>] \
  [-location <location>] \
  [-target-regions <region1,region2,...>]
```

The subscription is taken from the current Azure CLI configuration if not explicitly provided with `-subscription-id` or `AZURE_SUBSCRIPTION_ID`. The location is inferred from the resource-group, unless specified with `-location` or `AZURE_LOCATION`.

### Examples

Copy a community image into gallery `mygallery`, definition `mydef` and create version `0.0.1` in resource group `myrg`:

```bash
AZURE_LOCATION=eastus az-copy-image \
    -community-image-id "/CommunityGalleries/cococommunity-42d8482d-92cd-415b-b332-7648bd978eff/Images/peerpod-podvm-fedora-debug/Versions/0.12.0" \
    -image-gallery mygallery \
    -image-definition mydef \
    -resource-group myrg
```

Select target regions explicitly:

```bash
az-copy-image \
  ...
  -target-regions westeurope,eastus \
  -community-image-id /CommunityGalleries/....
```

Environment variables `AZURE_SUBSCRIPTION_ID`, `AZURE_RESOURCE_GROUP` and `AZURE_LOCATION` can be used to override the defaults.
