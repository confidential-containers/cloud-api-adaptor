# Setup procedure for IBM Cloud

This guide describes how to set up a demo environment on IBM Cloud for peer pod VMs.

The setup procedure includes the following sub tasks.

* Create a Virtual Private Cloud (VPC) including security groups, subnet, and gateway
* Create a Kubernetes cluster on two virtual server instances (VSIs)
* Build a custom VM image for pod VMs
* Install cloud-api-adaptor on a worker node
* Run a demo

## Prerequisites

To automate preparation of VPC and VSIs, you need to install terraform and ansible on your client machine. Please follow the the official installation guides.

* [Install Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli)
* [Install Ansible](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html)

Optionally, you can install IBM Cloud CLI.

* [Installing the stand-alone IBM Cloud CLI](https://cloud.ibm.com/docs/cli?topic=cli-install-ibmcloud-cli)

Note that you can use the IBM Cloud Web UI for most of the operations of IBM Cloud.

* [https://cloud.ibm.com/vpc-ext/overview](https://cloud.ibm.com/vpc-ext/overview)

You need IBM Cloud API key. You can create your own API key at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).

## Create a VPC

First, you need to create a Virtual Private Cloud (VPC). The Terraform configuration files are in [ibmcloud/terraform/common](./terraform/common/).

To use the Terraform configuration, you need to create a file `terraform.tfvars` at in the same directory of the other files of the Terraform configuration to specify your IBM Cloud API Key. `terraform.tfvars` looks like this.
```
ibmcloud_api_key = "<your API key>"
```
You can also customize the other parameters by specifying custom values in `terraform.tfvars`. The default values of such parameters are defined in [variables.tf](./terraform/common/variables.tf)

Then, you can create your VPC by executing the following commands.

```
cd ibmcloud/terraform/common
terraform init
terraform plan
terraform apply
```

The following cloud resources will be created. Please check [main.tf](terraform/common/main.tf) for the details.
* VPC
* Security groups
* Subnets
* Public gateway
* Floating IP for the public gateway

## Create a Kubernetes cluster

Another Terraform configuration is available at [ibmcloud/terraform/cluster](./terraform/cluster) to create a
Kubernetes cluster on the VPC you just created. Note that you can create multiple clusters by using different cluster names.

As usual, you need to create `terraform.tfvars` to specify parameter values. `terraform.tfvars` looks like this.

```
ibmcloud_api_key = ibmcloud_api_key = "<your API key>"
ssh_key_name = "<your SSH key name>"
cluster_name = "<cluster name>"
```

`ssh_key_name` is a name of your SSH key registered in IBM Cloud.
You can add your SSH key at [https://cloud.ibm.com/vpc-ext/compute/sshKeys](https://cloud.ibm.com/vpc-ext/compute/sshKeys). This ssh key will be installed on control-plane and worker nodes.

`cluster_name` is a name of a Kubernetes cluster. This name is used for the prefix of the names of control-plane and worker nodes. If you want to create another cluster in the same VPC, you need to use a different name for the new cluster.

Then, execute the following commands to create a new Kubernetes cluster consisting of two virtual server instances. One for a control-plane node, and another one for a worker node. Please check [main.tf](terraform/cluster/main.tf) for the details.

```
cd ibmcloud/terraform/cluster
terraform init
terraform plan
terraform apply
```

This Terraform configuration also triggers execution of an Ansible playbook to set up
Kubernetes and other prerequisite software in the two nodes. Please check [ansible/playbook.yml](terraform/cluster/ansible/playbook.yml) for the details.

When two VSIs are successfully provisioned, a floating IP address is assigned to the worker node. Please use the floating IP address to access the worker node from the Internet. You can check the floating IP using Web UI [https://cloud.ibm.com/vpc-ext/compute/vs](https://cloud.ibm.com/vpc-ext/compute/vs).

## Build a custom VM image

You need to build a custom VM image for pod VMs. A custom VM image contains the following components.

* Kata agent
* Agent protocol forwarder
* skopeo
* umoci

The build scripts are located in [ibmcloud/image](./image). The prerequisite software to build a custom VM image is already installed in the worker node by [the Ansible playbook](terraform/cluster/ansible/playbook.yml).

The following command builds a custom VM image.
```
cd ibmcloud/image
make build
```

You need to configure Cloud Object Storage (COS) to upload your custom VM image.

https://cloud.ibm.com/objectstorage/

First, create a COS service instance if you have not create one. Then, create a COS bucket with the COS instance. The COS service instance and bucket names are necessary to upload a custom VM image.

The following environment variables are necessary to be set before executing the image upload script. You can change IBMCLOUD_COS_REGION if you prefer another region. In this case, you also want to change IBMCLOUD_COS_SERVICE_ENDPOINT to one of endpoints listed at [https://cloud.ibm.com/docs/cloud-object-storage?topic=cloud-object-storage-endpoints](https://cloud.ibm.com/docs/cloud-object-storage?topic=cloud-object-storage-endpoints).

```
export IBMCLOUD_API_KEY=<your API key>
export IBMCLOUD_COS_SERVICE_INSTANCE=<COS service instance name>
export IBMCLOUD_COS_BUCKET=<COS bucket name>
export IBMCLOUD_COS_REGION=jp-tok
export IBMCLOUD_COS_SERVICE_ENDPOINT=https://s3.jp-tok.cloud-object-storage.appdomain.cloud
```

Then, you can execute the image upload script by using `make`.

```
make upload
```

After successfully uploading an image, you can verify the image by creating a virtual server instance using it. The following command will create a new server, and delete it. The VPC and subnet name are available in the terraform configuration mentioned above. You need to change the zone name if you have changed the region.

```
export IBMCLOUD_VPC_NAME=<VPC name>
export IBMCLOUD_VPC_SUBNET_NAME=<subnet name>
export IBMCLOUD_VPC_ZONE=jp-tok-2

make verify
```

Note that creating a server from a new image may take long time. It typically takes about 10 minutes. From the second time, creating a server from the image takes one minute.

You can check the name and ID of the new image at [https://cloud.ibm.com/vpc-ext/compute/images](https://cloud.ibm.com/vpc-ext/compute/images).
