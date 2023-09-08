# Setup instructions
## Prerequisites

- Set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` for AWS cli access

- Install packer by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)

- Install packer's Amazon plugin `packer plugins install github.com/hashicorp/amazon`

## Build Pod VM Image

### Option-1: Modifying existing marketplace image

- Set environment variables
```
export AWS_REGION="us-east-1" # mandatory
export PODVM_DISTRO=rhel # mandatory
export INSTANCE_TYPE=t3.small # optional, default is t3.small
export IMAGE_NAME=peer-pod-ami # optional
export VPC_ID=vpc-01234567890abcdef # optional, otherwise, it creates and uses the default vpc in the specific region
export SUBNET_ID=subnet-01234567890abcdef # must be set if VPC_ID is set
```

If you want to change the volume size of the generated AMI, then set the `VOLUME_SIZE` environment variable.
For example if you want to set the volume size to 40 GiB, then do the following:
```
export VOLUME_SIZE=40
```

- Create a custom AWS VM image based on Ubuntu 22.04 having kata-agent and other dependencies

> **NOTE**: For setting up authenticated registry support read this [documentation](../docs/registries-authentication.md).

```
cd image
make image
```

You can also build the custom AMI by running the packer build inside a container:

```
docker build -t aws \
--secret id=AWS_ACCESS_KEY_ID \
--secret id=AWS_SECRET_ACCESS_KEY \
--build-arg AWS_REGION=${AWS_REGION} \
-f Dockerfile .
```

If you want to use an existing `VPC_ID` with public `SUBNET_ID` then use the following command:
```
docker build -t aws \
--secret id=AWS_ACCESS_KEY_ID \
--secret id=AWS_SECRET_ACCESS_KEY \
--build-arg AWS_REGION=${AWS_REGION} \
--build-arg VPC_ID=${VPC_ID} \
--build-arg SUBNET_ID=${SUBNET_ID}\
-f Dockerfile .
```

If you want to build a CentOS based custom AMI then you'll need to first
accept the terms by visiting this [link](https://aws.amazon.com/marketplace/pp?sku=bz4vuply68xrif53movwbkpnl)

Once done, run the following command:

```
docker build -t aws \
--secret id=AWS_ACCESS_KEY_ID \
--secret id=AWS_SECRET_ACCESS_KEY \
--build-arg AWS_REGION=${AWS_REGION} \
--build-arg BINARIES_IMG=quay.io/confidential-containers/podvm-binaries-centos-amd64 \
--build-arg PODVM_DISTRO=centos \
-f Dockerfile .
```

- Note down your newly created AMI_ID

Once the image creation is complete, you can use the following CLI command as well to
get the AMI_ID. The command assumes that you are using the default AMI name: `peer-pod-ami`

```
aws ec2 describe-images --query "Images[*].[ImageId]" --filters "Name=name,Values=peer-pod-ami" --region ${AWS_REGION} --output text
```

### Option-2: Using precreated QCOW2 image

- Download QCOW2 image
```
mkdir -p qcow2-img && cd qcow2-img

curl -LO https://raw.githubusercontent.com/confidential-containers/cloud-api-adaptor/staging/podvm/hack/download-image.sh

bash download-image.sh quay.io/confidential-containers/podvm-generic-ubuntu-amd64:latest . -o podvm.qcow2

```

- Convert QCOW2 image to RAW format
You'll need the `qemu-img` tool for conversion.
```
qemu-img convert -O raw podvm.qcow2 podvm.raw
```

- Upload RAW image to S3 and create AMI
You can use the following helper script to upload the podvm.raw image to S3 and create an AMI
Note that AWS cli should be configured to use the helper script.

```
curl -L0 https://raw.githubusercontent.com/confidential-containers/cloud-api-adaptor/staging/aws/raw-to-ami.sh

bash raw-to-ami.sh podvm.raw <Some S3 Bucket Name> <AWS Region>
```

On success, the command will generate the `AMI_ID`, which needs to be used to set the value of `PODVM_AMI_ID` in `peer-pods-cm` configmap.

## Running cloud-api-adaptor

- Update [kustomization.yaml](../install/overlays/aws/kustomization.yaml) with your AMI_ID

- Deploy Cloud API Adaptor by following the [install](../install/README.md) guide
