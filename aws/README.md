# Setup instructions

- Install packer by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)
	- Install packer's Amazon plugin `packer plugins install github.com/hashicorp/amazon`

- Create a VPC with public internet access
Note down the VPC ID, Subnet ID, Region

- Set environment variables
```
export AWS_ACCOUNT_ID="REPLACE_ME"
export VPC_ID="REPLACE_ME"
export SUBNET_ID="REPLACE_ME"
export AWS_REGION="REPLACE_ME"
```

- Create a custom AMI based on Ubuntu 20.04 having kata-agent and other dependencies.
```
cd image
CLOUD_PROVIDER=aws make build
```

- Create an EC2 launch template named "kata" using the newly created AMI



# Running cloud-api-adaptor

- Update [kustomization.yaml](../install/overlays/aws/kustomization.yaml) with the required values.

- Deploy Cloud API Adaptor by following the [install](../install/README.md) guide.

