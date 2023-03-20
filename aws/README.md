# Setup instructions
## Prerequisites

- Set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` for AWS cli access

- Install packer by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)
        - Install packer's Amazon plugin `packer plugins install github.com/hashicorp/amazon`

## Image Build

- Set environment variables
```
export AWS_REGION="us-east-1" # mandatory
export PODVM_DISTRO=rhel # mandatory
export INSTANCE_TYPE=t3.small # optional, default is t3.small
export IMAGE_NAME=peer-pod-ami-image # optional
export VPC_ID=vpc-01234567890abcdef # optional, otherwise, it creates and uses the default vpc in the specific region
export SUBNET_ID=subnet-01234567890abcdef # must be set if VPC_ID is set
```

- Create a custom AMI based on Ubuntu 20.04 having kata-agent and other dependencies
	- [setting up authenticated registry support](../docs/registries-authentication.md)
```
cd image
make image
```

- Note down your newly created AMI_ID

## Running cloud-api-adaptor

- Update [kustomization.yaml](../install/overlays/aws/kustomization.yaml) with your AMI_ID

- Deploy Cloud API Adaptor by following the [install](../install/README.md) guide
