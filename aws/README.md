# Setup instructions
## Prerequisites

- Set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` for AWS cli access

- Install packer by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)
        - Install packer's Amazon plugin `packer plugins install github.com/hashicorp/amazon`

## Image Build

- Set environment variables
```
export AWS_ACCOUNT_ID="REPLACE_ME"
export AWS_REGION="REPLACE_ME"
```

- Either make sure default VPC is enabled: `aws ec2 create-default-vpc --region ${AWS_REGION}` or
create create new VPC with public internet access and set also the following environment variables
```
export VPC_ID="REPLACE_ME"
export SUBNET_ID="REPLACE_ME"
```
   
- Create a custom AMI based on Ubuntu 20.04 having kata-agent and other dependencies
	- [setting up authenticated registry support](../docs/registries-authentication.md)
```
cd image
CLOUD_PROVIDER=aws make image
```

- Note down your newly created AMI_ID

## Running cloud-api-adaptor

- Update [kustomization.yaml](../install/overlays/aws/kustomization.yaml) with your AMI_ID

- Deploy Cloud API Adaptor by following the [install](../install/README.md) guide
