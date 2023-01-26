# Importing Public PODVM images into IBM Cloud VPC

As part of the release process pre-built images are published as container images to the confidential-containers quay repository. e.g. `quay.io/confidential-containers/podvm-ibmcloud-ubuntu-s390x`. These contain a single qcow2 file that needs to be extracted and uploaded to ibmcloud to use as a vpc image.

To simpify this process a script has been created to aid this. `ibmcloud/image/import.sh`.

## Prerequisites

### Tools

- jq `apt install jq`
- ibmcloud `curl -fsSL https://clis.cloud.ibm.com/install/linux | sh` (https://cloud.ibm.com/docs/cli?topic=cli-getting-started)
- docker/podman `apt install docker.io`

### Cloud-Object-Storage

To create the VPC image you need to first import the file to ibmcloud COS. The script will do this step but a bucket must already be available.

You may follow the [offical documentation](https://cloud.ibm.com/docs/cloud-object-storage?topic=cloud-object-storage-getting-started-cloud-object-storage) to create a bucket, a free tier is sufficient.

## Running

### Example

`./import.sh quay.io/confidential-containers/podvm-ibmcloud-ubuntu-s390x ca-tor --instance jt-cos-instance --bucket podvm-image-cos-bucket-jt --region jp-tok`

### Arguments

The first two arguments are positional:

1. The name of the image to import
1. The VPC region to create the image in (the process can be repeated for multiple regions)

The later options:

- `instance`: the cos instance name. Required if the cli has not been configured with the cos instance before
- `bucket`: name of the bucket to use. Will use an available bucket if not specified
- `region`: name of the region the bucket is in. Only required if that region is different from the VPC region
