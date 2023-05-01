# Setup instructions
## Prerequisites

- Set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` for AWS cli access

- Install packer by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)

- Install packer's Amazon plugin `packer plugins install github.com/hashicorp/amazon`

## Build Pod VM Image

- Set environment variables
```
export AWS_REGION="us-east-1" # mandatory
export PODVM_DISTRO=rhel # mandatory
export INSTANCE_TYPE=t3.small # optional, default is t3.small
export IMAGE_NAME=peer-pod-ami # optional
export VPC_ID=vpc-01234567890abcdef # optional, otherwise, it creates and uses the default vpc in the specific region
export SUBNET_ID=subnet-01234567890abcdef # must be set if VPC_ID is set
```

- Create a custom AWS VM image based on Ubuntu 20.04 having kata-agent and other dependencies

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

## Running cloud-api-adaptor

- Update [kustomization.yaml](../install/overlays/aws/kustomization.yaml) with your AMI_ID

- Deploy Cloud API Adaptor by following the [install](../install/README.md) guide
