# Setup procedure for IBM Cloud

This guide describes how to set up a demo environment on IBM Cloud for peer pod VMs.

This procedure has been confirmed using the following repositories.
* https://github.com/confidential-containers/cloud-api-adaptor/tree/staging
* https://github.com/kata-containers/kata-containers/tree/CCv0

The setup procedure includes the following sub tasks.

* Create a Virtual Private Cloud (VPC) including security groups, subnet, and gateway
* Create a Kubernetes cluster on two virtual server instances (VSIs)
* Build a custom VM image for pod VMs
* Install cloud-api-adaptor on a worker node
* Run a demo

## Prerequisites

To automate preparation of VPC and VSIs, you need to install terraform and ansible on your `development machine`. Please follow the the official installation guides.

* [Install Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli)

> **Tip:** If you are using Ubuntu linux, you can run follow commands simply:
> ```bash
> $ sudo apt-get update && sudo apt-get install -y gnupg software-properties-common curl
> $ curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
> $ sudo apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
> $ sudo apt-get install terraform -y
> ```

* [Install Ansible](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html)

> **Tip:** If you are using Ubuntu linux, you can run follow commands simply:
> ```bash
> $ sudo apt-get install -y python3
> $ sudo ln -s /usr/bin/python3 /usr/bin/python
> $ sudo add-apt-repository --yes --update ppa:ansible/ansible
> $ sudo apt-get install ansible -y
> ```

Optionally, you can install IBM Cloud CLI.

* [Installing the stand-alone IBM Cloud CLI](https://cloud.ibm.com/docs/cli?topic=cli-install-ibmcloud-cli)

> **Tips**
> - If you are using Ubuntu linux, you can run follow commands simply:
>     ```bash
>     $ curl -fsSL https://clis.cloud.ibm.com/install/linux | sh
>     $ ibmcloud plugin install vpc-infrastructure cloud-object-storage
>     ```
> - You can use the [IBM Cloud Web UI](https://cloud.ibm.com/vpc-ext/overview) for most of the operations of IBM Cloud. And please make sure that you are selecting the correct region in the Web UI.
> 

* You need IBM Cloud API key. You can create your own API key at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).

## Create and test the demo environment on IBM Cloud for peer pod VMs - Basic Usage

### End to end Terraform configuration

You can create the demo environment for peer pod VMs on IBM Cloud Virtual Private Cloud (VPC) with the Terraform configuration located in [ibmcloud/terraform](./terraform). This Terraform configuration will:

* Set up VPC infrastructure network resources, including the VPC, subnets and security groups
* Set up VPC infrastructure compute resources, including virtual server instances for the Kubernetes control plane and worker
* Optionally import a SSH public key to VPC Infrastructure
* Build and install software package dependencies on the Kubernetes control plane and worker instances
* Configure the Kubernetes control plane and worker instances
* Build the peer pod VM image on the Kubernetes worker instance
* Set up IBM Cloud Object Storage (COS) resources
* Push the built peer pod VM image to COS and import it from COS to VPC Infrastructure as a custom image
* Start the `cloud-api-adaptor` process on the Kubernetes worker instance
* Create an nginx pod that runs on a peer pod instance on the Kubernetes cluster
* Test the nginx peer pod

The Terraform configuration supports building the demo environment on both the x86 (Intel) and s390x (IBM Z) architectures.

To use the Terraform configuration, you need to create a file `terraform.tfvars` in the [configuration directory](./terraform) to specify parameters for the Terraform configuration. The `terraform.tfvars` file with all mandatory parameters looks like this:

```
ibmcloud_api_key = "<your API key>"
ibmcloud_user_id = "<your IBM Cloud User ID>"
cluster_name = "<cluster name>"
ssh_key_name = "<name of your SSH key>"
podvm_image_name = "<name of the peer pod VM image to build>"
cos_bucket_name = "<name of the COS bucket name to be created>"
```

When all parameters are specified the `terraform.tfvars` will have the additional lines:

```
region_name = "<name of an IBM Cloud region>"
zone_name = "<name of a zone in your IBM Cloud zone region>"
ssh_pub_key = "<your SSH public key>"
cos_service_instance_name = "<name of the COS instance to create>"
cos_bucket_region = "<name of the region to create the COS bucket in>"
floating_ip_name = "<name of the floating IP to create>"
image_name = "<name of the image to use for the Kubernetes control plane and worker>"
instance_profile_name = "<name of the instance profile to use for the Kubernetes control plane and worker>"
primary_security_group_name = "<name of the primary security group to create>"
primary_subnet_name = "<name of the primary subnet name to create>"
public_gateway_name = "<name of the public gateway name to create>"
vpc_name = "<vpc name>"
```

#### Parameters

> **Notes:**
> - `ibmcloud_api_key` is your IBM Cloud API key that you created at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).
> - `region_name` (optional) is the IBM Cloud region Terraform will create the demo environment in. If not set it defaults to `jp-tok`.
> - `ibmcloud_user_id` is the IBM Cloud user ID who owns the API key specified using the `ibmcloud_api_key` parameter. You can look up the user ID by running
>     ```bash
>     $ ibmcloud login --apikey <API key used for the ibmcloud_api_key parameter> -r <region name used for the region_name parameter>
>     $ ibmcloud account users
>     ```
>     If command `ibmcloud account users` displays multiple user IDs, choose the user ID whose state is `ACTIVE`.
> - `cluster_name` is a name of a Kubernetes cluster. This name is used for the prefix of the names of control plane and worker node virtual server instances.
> - `ssh_key_name` is the name of your SSH key registered in IBM Cloud or the name of a new SSH key if a public key is also provided using the optional `ssh_pub_key` parameter. You can add your SSH key at [https://cloud.ibm.com/vpc-ext/compute/sshKeys](https://cloud.ibm.com/vpc-ext/compute/sshKeys). For more information about SSH keys see [managing SSH Keys](https://cloud.ibm.com/docs/vpc?topic=vpc-ssh-keys). The SSH key will be installed on the Kubernetes control plane and worker nodes and is used to access them from your `development machine`.
> - `podvm_image_name` is the name of the VPC infrastructure custom image for the peer pod VM that the Kubernetes worker will build. This name will have `-amd64` or `-s390x` appended to it to name the image that eventually gets imported to VPC infrastructure custom images, depending on if the `image_name` parameter for the instance the peer pod VM image is built on uses the amd64 or s390x CPU architecture
> - `cos_bucket_name` is the name of the COS bucket that will store the peer pod .vsi image. This bucket name must be unique across all IBM Cloud accounts.
> - `cos_bucket_region` (optional) is the name of the region that the COS bucket will be in, this can be regional (e.g. jp-tok) or cross-regional (e.g. eu). If not provided will be the region specified by `region_name`.
> - `zone_name` (optional) is the zone in the region Terraform will create the demo environment in. If not set it defaults to `jp-tok-2`.
> - `ssh_pub_key` (optional) is an variable for a SSH public key which has **not** been registered in IBM Cloud in the targeted region. Terraform will manage this key instead. You cannot register the same SSH public key in the same region twice under different SSHs key names.
> - `cos_service_instance_name` (optional) is the name of the COS service instance Terraform will create. If not set it defaults to `cos-image-instance`.
> - `floating_ip_name` (optional) is the name of the floating IP that is assigned to the Kubernetes worker. If not set it defaults to `tok-gateway-ip`.
> - `image_name` (optional) is a name of IBM Cloud infrastructure image. This name is used to create virtual server instances for the Kubernetes control plane and worker. For more information, about VPC custom images, see [IBM Cloud Importing and managing custom images](https://cloud.ibm.com/docs/vpc?topic=vpc-managing-images). If not set it defaults to `ibm-ubuntu-20-04-3-minimal-amd64-1`.
> - `resource_group_id` (optional) is the resource group ID in IBM Cloud, under which the peer pod will be created. If not set it defaults to your default resource group.
> - `instance_profile_name` (optional) is a name of IBM Cloud virtual server instance profile. This name is used to create virtual server instances for the Kubernetes control plane and worker. For more information, about virtual server instance profile, see [instance profiles](https://cloud.ibm.com/docs/vpc?topic=vpc-profiles). If not set it defaults to `bx2-2x8`, which uses the amd64 architecture, has 2 vCPUs and 8 GB memory.
> - `primary_security_group_name` (optional) is the name of the security group Terraform will create. If not set it defaults to `tok-primary-security-group`.
> - `primary_subnet_name` (optional) is the name of the subnet Terraform will create. If not set it defaults to `tok-primary-subnet`.
> - `public_gateway_name` (optional) is the name of the public gateway Terraform will create. If not set it defaults to `tok-gateway`.
> - `vpc_name` (optional) is the name of the VPC Terraform will create. If not set it defaults to `tok-vpc`.
> - `cloud_api_adaptor_repo` (optional) is the repository URL of Cloud API Adaptor. If not set it defaults to `https://github.com/confidential-containers/cloud-api-adaptor.git`.
> - `cloud_api_adaptor_branch` (optional) is the branch name of Cloud API Adaptor. If not set it defaults to `staging`.
> - `kata_containers_repo` (optional) is the repository URL of Kata Containers. If not set it defaults to `https://github.com/kata-containers/kata-containers.git`.
> - `kata_containers_branch` (optional) is the branch name of Kata Containers. If not set it defaults to `CCv0`.
> - `containerd_repo` (optional) is the repository URL of containerd. If not set it defaults to `https://github.com/confidential-containers/containerd.git`.
> - `containerd_branch` (optional) is the branch name of containerd. If not set it defaults to `CC-main`.

> **Hint:** In order to create a cluster based on a different type of VSI image you can use the `instance_profile_name` and `image_name` parameters. E.g., to create a **s390x** architecture based cluster, include the following two lines in the `terraform.tfvars` file
>
>     instance_profile_name = "bz2-2x8"
>     image_name = "ibm-ubuntu-18-04-1-minimal-s390x-3"
>

After writing you `terraform.tfvars` file you can create your VPC by executing the following commands on your `development machine`:
```bash
$ cd ibmcloud/terraform
$ terraform init
$ terraform plan
$ terraform apply
```

The following IBM Cloud resources will be created when running the end-to-end Terraform configuration. Please check the `main.tf` file in each subdirectory of [ibmcloud/terraform/](./terraform) for details regarding which resources each individual module creates.
* VPC
* Security groups
* Subnets
* Public gateway
* Floating IP for the public gateway
* Virtual server instances for the Kubernetes control plane and worker
* Floating IPs for the Kubernetes control plane and worker virtual server instances
* COS Instance with 1 COS bucket
* Custom image for the peer pod VM image
* Virtual server instance for the peer pod running the nginx workload
* SSH key, if you specified the optional `ssh_pub_key` variable

On a cluster using `instance_profile_name = "bz2-2x8"` and `image_name = "ibm-ubuntu-18-04-1-minimal-s390x-3"` the end-to-end playbook takes approximately 50 minutes to complete. 

## Create and test the demo environment on IBM Cloud for peer pod VMs - advanced usage

The individual modules this Terraform configuration calls can also be ran as stand-alone Terraform configurations. This is recommended for experienced users who want to try to set up the demo environment on pre-existing infrastructure.

### Create a VPC

First, you need to create a VPC. The Terraform configuration files are in [ibmcloud/terraform/common](./terraform/common/).

To use the Terraform configuration, you need to create a file `terraform.tfvars` at in the same directory of the other files of the Terraform configuration to specify your IBM Cloud API Key. The `terraform.tfvars` looks like this.
```
ibmcloud_api_key = "<your API key>"
```

You can also customize the other parameters by specifying custom values in `terraform.tfvars`. The default values of such parameters are defined in [variables.tf](./terraform/common/variables.tf)

Then, you can create your VPC by executing the following commands on your `development machine`.

```bash
$ cd ibmcloud/terraform/common
$ terraform init
$ terraform plan
$ terraform apply
```

### Create a Kubernetes cluster

Another Terraform configuration is available at [ibmcloud/terraform/cluster](./terraform/cluster) to create a Kubernetes cluster on the VPC you just created, or on a pre-existing VPC. This configuration is called as a Terraform module by the end-to-end configuration, but it can be ran as a stand-alone Terraform configuration.

> **Tip:** You can create multiple clusters by using different cluster names.

As usual, you need to create `terraform.tfvars` to specify parameter values. The `terraform.tfvars` looks like this.

```
ibmcloud_api_key = "<your API key>"
ssh_key_name = "<your SSH key name>"
cluster_name = "<cluster name>"
primary_subnet_name = "<name of your primary subnet>" OR primary_subnet_id = "<ID of your primary subnet>"
primary_security_group_name = "<name of your primary security group>" OR primary_security_group_id = "<ID of your primary security group>"
vpc_name = "<name of your VPC>" OR vpc_id = "<ID of your VPC>"
```

If you created your VPC, subnet and security group without using the `common` Terraform configuration you should provide the name or ID of your existing resources as the `primary_subnet_name/primary_subnet_id`, `primary_security_group_name/primary_security_group_id` and `vpc_name/vpc_id` parameters. If you created your VPC, subnet and security group using the `common` Terraform configuration you can find the IDs of your VPC, subnet and security group in the outputs of the `common` Terraform configuration.

If you don't have your public key already configured in IBM Cloud you can add
```
ssh_pub_key = "<your SSH public key>"
```

Additionally,  you can customize source code repositories to be extracted under `/root` of each worker node. By default, the repository URLs and branch names listed below are used to fetch repositories. You can add variable definitions to `terraform.tfvars` to customize them.

```
cloud_api_adaptor_repo = "https://github.com/confidential-containers/cloud-api-adaptor.git"
cloud_api_adaptor_branch = "staging"
kata_containers_repo = "https://github.com/kata-containers/kata-containers.git"
kata_containers_branch = "CCv0"
containerd_repo = "https://github.com/confidential-containers/containerd.git"
containerd_branch = "CC-main"
```

> **Hint:** In order to create the cluster based on a different type of VSI image you can overwrite more parameters here e.g. to create a **s390x** based cluster add follow two lines to the `terraform.tfvars` file.
>
>     instance_profile_name = "bz2-2x8"
>     image_name = "ibm-ubuntu-18-04-1-minimal-s390x-3"
>
> **Notes:**
> - Some resources can be specified using their name or ID. For example, the subnet can be specified using the `primary_subnet_name` or `primary_subnet_id` variables. Where this option exists the `..._name` and `..._id` variables are mutually exclusive.
> - Resources that can be specified using either their name or ID must exist when the Terraform configuration is planned or applied.
> - Variables with the same name as variables in the end to end Terraform configuration are as described in that [configuration's parameters](#parameters).
> - If you want to create more than one cluster in the same VPC, you need to use a different `cluster_name` for each cluster.
> - Additional variables and their defaults are defined in the [variables.tf](./terraform/cluster/variables.tf) file for this Terraform configuration

Then, execute the following commands to create a new Kubernetes cluster consisting of two Virtual server instances. One for a control plane node, and another one for a worker node. Please check [main.tf](terraform/cluster/main.tf) for the details.

```bash
$ cd ibmcloud/terraform/cluster
$ terraform init
$ terraform plan
$ terraform apply
```

> **Tip:** You can check the status of provisioned Kubernetes node VM instances at [https://cloud.ibm.com/vpc-ext/compute/vs](https://cloud.ibm.com/vpc-ext/compute/vs).

The SSH key installed on control-plane and worker nodes will be output at the end of the terraform configuration.

```bash
Outputs:
ssh_key_name = <SSH key name>
```

This Terraform configuration also triggers execution of two Ansible playbooks to set up Kubernetes and other prerequisite software in the two nodes. Please check [ansible/kube-playbook.yml](terraform/cluster/ansible/kube-playbook.yml) and [ansible/kata-playbook.yml](terraform/cluster/ansible/kata-playbook.yml) for the details.

If ansible fails for some reason, you can rerun the Ansible playbooks as follows.
```bash
$ cd ansible
$ ansible-playbook -i inventory -u root ./kube-playbook.yml && ansible-playbook -i inventory -u root ./kata-playbook.yml
```

When ansible fails, Terraform does not execute the setup script for Kubernetes. In this case, you can manually run it as follows.

```bash
$ ./scripts/setup.sh --bastion <floating IP of the worker node> --control-plane <IP address of the control-plane node> --workers  <IP address of the worker node>
```

> **Note:** You do not need to run this script manually, when everything goes well.
As there is only a single node. All of the rest look correct though!

When two VSIs are successfully provisioned, a floating IP address is assigned to the worker node. You can use the floating IP address to access the worker node from the Internet, or to ssh into the worker node from your `development machine`:
```bash
$ ssh root@floating-ip-of-worker-node
```

### Build a pod VM image

You need to build a pod VM image for peer pod VMs. A pod VM image contains the following components.

* Kata agent
* Agent protocol forwarder
* skopeo
* umoci

The build scripts are located in [ibmcloud/image](./image). The prerequisite software to build a pod VM image is already installed in the worker node by [the Ansible playbook](terraform/cluster/ansible/playbook.yml) for convenience.

You need to configure Cloud Object Storage (COS) to upload your custom VM image.

https://cloud.ibm.com/objectstorage/


First, create a COS service instance if you have not create one. Then, create a COS bucket with the COS instance. The COS service instance and bucket names are necessary to upload a custom VM image.

You can use the Terraform configuration located at [ibmcloud/terraform/cos](./terraform/cos) to use Terraform to create a COS service instance, COS bucket, and IAM Authorization Policy automatically. These resources are configured to store the peer pod VM images. Create a `terraform.tfvars` file in the configurations directory that includes these fields:

```
ibmcloud_api_key = "<your API key>"
cos_bucket_name = "<name of the COS bucket to create>"
cos_service_instance_name = "<name of the COS instance to create>"
cos_bucket_region = "<name of the region to create the COS bucket in>"
```

> **Notes:**
> - Variables with the same name as variables in the end to end Terraform configuration are as described in that [configuration's parameters](#parameters).
> - Additional variables and their defaults are defined in the [variables.tf](./terraform/cos/variables.tf) file for this Terraform configuration.
> - The bucket can be regional or cross-regional see [Endpoints & Locations](https://cloud.ibm.com/docs/cloud-object-storage?topic=cloud-object-storage-endpoints)

Then run the Template via the following commands: 
```bash
$ cd ibmcloud/terraform/cos
$ terraform init
$ terraform plan
$ terraform apply
```

You can use a Terraform configuration located at [ibmcloud/terraform/podvm-build](./terraform/podvm-build) to use Terraform and Ansible to build a pod VM image on the k8s worker node, upload it to a COS bucket and verify it. The architecture of the pod VM image built on the k8s worker node will be the same as that of the node. For example, a k8s worker node using an Intel **x86** VSI will build an Intel **x86** pod VM image and a k8s worker node using an IBM **s390x** VSI will build an IBM **s390x** pod VM image.

> **Warning:** Building a pod VM image on a worker node using the Terraform configuration is not recommended for production, and we need to build a pod VM image somewhere secure to protect workloads running in a peer pod VM.

Create the `terraform.tfvars` file in [the configuration directory](./terraform/podvm-build). The `terraform.tfvars` looks like this.
```
ibmcloud_api_key = "<your API key>"
ibmcloud_user_id = "<IBM Cloud User ID>"
cluster_name = "<cluster name>"
cos_service_instance_name = "<Name of your COS service instance>" OR cos_service_instance_id = "<ID of your COS service instance>"
cos_bucket_name = "<Name of your COS bucket name>"
cos_bucket_region = "<name of the region to create the COS bucket in>"
```

If you created your COS resources without using with `cos` Terraform configuration you should provide the name or ID of your existing COS service instance as the `cos_service_instance_name/cos_service_instance_id` parameter and the name of your existing COS bucket as the `cos_bucket_name` parameter. If you created your COS service instance `cos` Terraform configuration you can find the ID of your COS service instance in the outputs of the `cos` Terraform configuration.

> **Notes:**
> - The `cos_service_instance` resource can be specified using its name or ID. The `cos_service_instance_name` and `cos_service_instance_id` variables are mutually exclusive.
> - Variables with the same name as variables in the end to end Terraform configuration are as described in that [configuration's parameters](#parameters).
> - All COS and VPC infrastructure resources must exist before running this Terraform configuration.
> - Additional variables and their defaults are defined in the [variables.tf](./terraform/podvm-build/variables.tf) file for this Terraform configuration.
> - If you don't specify the optional `podvm_image_name` variable then the name of the custom image created will be based on [the latest commit hash](https://github.com/confidential-containers/cloud-api-adaptor/commits/staging) of the confidential containers cloud API adaptor staging branch.
> - If you specify `podvm_image_name` yourself you must add the `-amd64` or `-s390x` suffix to `podvm_image_name` depending on the CPU architecture of the instance you are building the peer pod VM image on. This is different to the behaviour of this parameter in the e2e Terraform configuration.
> - The `Operator` and `Console Admin` roles must be [assigned](https://cloud.ibm.com/docs/vpc?topic=vpc-vsi_is_connecting_console&interface=ui) to the user. The Terraform configuration will create the `Console Admin` role for the user `ibmcloud_user_id` is set to in the configuration `terraform.tfvars`.

Execute the following commands on your `development machine` to build, upload and verify the pod VM image.

```bash
$ cd ibmcloud/terraform/podvm-build
$ terraform init
$ terraform plan
$ terraform apply
```

> **Notes:** 
> - If your worker node is **s390x** based, the suffix of the created QCOW2 file for the custom image will be `-s390x` otherwise it will be `-amd64`.
> - It typically takes about 15~20 minutes for task `Build peer pod VM image`
> - It typically takes about 7~10 minutes for task `Push peer pod VM image to Cloud Object Store and verify the image`
> - After all tasks finish, when you creating a server from the image it will only takes 1~5 minutes.
> - You can check the name and ID of the new image at [https://cloud.ibm.com/vpc-ext/compute/images](https://cloud.ibm.com/vpc-ext/compute/images). Alternatively, you can use the `ibmcloud` command to list your images as follows.
>    ```bash
>    $ ibmcloud is images --visibility=private
>    ```

### Enabling Attestation agent and Authenticated Registry
**Prerequisites:**
- An ibmcloud worker node using the cloud-api-adaptor
- A [auth.json](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md] file with your credentials)
- SSH'd into the worker node: `ssh root@floating-ip-of-worker-node`

Once you have the prerequisites use [static registries authentication setup](../docs/registries-authentication.md#statically-embed-authentication-file-in-podvm-image)

**Building the image:**
- `cd ~/cloud-api-adaptor/ibmcloud/image`
- Export these variables:
```
export CLOUD_PROVIDER=ibmcloud
export IMAGE_NAME=<An image name with arch included e.g. image-amd64>
export IBMCLOUD_COS_REGION=<the region your IKS cluster and zVSI is running on>
export IBMCLOUD_VPC_REGION=$IBMCLOUD_COS_REGION
export IBMCLOUD_VPC_NAME=<your generated vpc name from the terraform>
export IBMCLOUD_VPC_SUBNET_NAME=<the linked subnet name to the above vpc>
export IBMCLOUD_COS_SERVICE_ENDPOINT="https://s3.${IBMCLOUD_COS_REGION}.cloud-object-storage.appdomain.cloud"
export IBMCLOUD_COS_SERVICE_INSTANCE=<your cos instance>
export IBMCLOUD_COS_BUCKET=<your cos bucket in the region your cluster runs in>
export IBMCLOUD_API_KEY=<your ibmcloud apikey>
export IBMCLOUD_API_ENDPOINT="https://cloud.ibm.com"
export GOPATH=/root/go
```
- run `AA_KBC="offline_fs_kbc" make push` 
> **Note:** The image ID can be found at the end of the logs when the make steps have completed. This can then be substituted for the image id in the start cloud api adaptor steps below.

## Install custom Kata shim

The Ansible playbook automatically installs the custom Kata shim binary and its configuration file to worker node. If you want to rebuild the Kata shim, please follow the steps below.

```bash
$ cd /root/kata-containers/src/runtime
$ make $PWD/containerd-shim-kata-v2
$ install containerd-shim-kata-v2 /usr/local/bin/
```

A minimum Kata shim configuration file at `/etc/kata-containers/configuration.toml` looks like this.

```
[runtime]
internetworking_model = "none"
disable_new_netns = true
disable_guest_seccomp = true
enable_pprof = true
enable_debug = true
[hypervisor.remote]
remote_hypervisor_socket = "/run/peerpod/hypervisor.sock"
[agent.kata]
```

## Install Cloud API adaptor

The Ansible playbook automatically installs the Cloud API adaptor binary to worker node. If you want to rebuild it, please follow the steps below.

```bash
$ cd /root/cloud-api-adaptor
$ CLOUD_PROVIDER=ibmcloud make
$ install cloud-api-adaptor /usr/local/bin/
```

## Launch Cloud API adaptor

A terraform configuration that will start the `cloud-api-adaptor` process on the Kubernetes worker node is available in [ibmcloud/terraform/start-cloud-api-adaptor](./terraform/start-cloud-api-adaptor).

Create a `terraform.tfvars` file in the [configuration directory](./terraform/start-cloud-api-adaptor) for this Terraform configuration on your `development machine`. The `terraform.tfvars` file should look like this

```
ibmcloud_api_key = "<your API Key>"
cluster_name = "<cluster name>"
ssh_key_name = "<your SSH key name>" OR ssh_key_id = "<your SSH key id>"
podvm_image_name = "<name of your pod VM image>" OR podvm_image_id = "<ID of your peer pod VM image>"
vpc_name = "<name of your VPC>" OR vpc_id = "<ID of your VPC>"
primary_subnet_name = "<name of your primary subnet>" OR primary_subnet_id = "<ID of your primary subnet>"
primary_security_group_name = "<name of your primary security group>" OR primary_security_group_id = "<ID of your primary security group>"
```

If you created your infrastructure using the other Terraform configurations included in this repo then you can find the ID of your SSH key in the output values of the `cluster` Terraform configuration, the ID of your peer pod VM image in the output values of the `podvm-image` Terraform configuration and the IDs of your VPC, subnet and security group in the output values of the `common` Terraform configuration. If you created these resources without using the Terraform configurations then you should find the names or IDs of these resources in your existing IBM Cloud infrastructure and provide them to the respective parameters.

> **Hints:**
> - The `instance_profile_name` optional variable sets the CPU architecture, number of vCPUs and memory of each peer pod virtual server instance. E.g., the `bz2-2x8` instance profile uses the s390x CPU architecture, has 2 vCPUs and 8 GiB of memory
> - If you created the cluster based on an s390x architecture VSI image you must set the `instance_profile_name` parameter to the name of an s390x-architecture instance profile. E.g., if your cluster uses the **s390x** CPU architecture add the following line to the `terraform.tfvars` file
>
>     instance_profile_name = "bz2-2x8"
>
> `bz2-2x8` can be replaced with the name of a different s390x-architecture instance profile

> **Notes:**
> - Some resources can be specified using their name or ID. Where this option exists the `..._name` and `..._id` variables are mutually exclusive.
> - Variables with the same name as variables in the end to end Terraform configuration are as described in that [configuration's parameters](#parameters).
> - Additional variables and their defaults are defined in the [variables.tf](./terraform/start-cloud-api-adaptor/variables.tf) file for this Terraform configuration.
> - To view [IBM Cloud Custom images for VPC](https://cloud.ibm.com/vpc-ext/compute/images) for your chosen region to view the name of the pod VM custom image that was built and uploaded, or run the command `ibmcloud is images --visibility=private`.

Execute the following commands on your `development machine` to start the cloud api adaptor on your worker instance:

```bash
$ cd ibmcloud/terraform/start-cloud-api-adaptor
$ terraform init
$ terraform plan
$ terraform apply
```

After `terraform apply` completes the `cloud-api-adaptor` process will run on the Kubernetes worker instance until it is deleted. See the subsection [delete the demo configuration and pod](./README.md#delete-the-demo-configuration-and-pod).

## Demo

### Deploy the nginx pod and sniff test nginx

A Terraform configuration that will deploy an nginx pod to the Kubernetes cluster is available in [ibmcloud/terraform/run-nginx-demo](./terraform/run-nginx-demo). This configuration will also check the nginx peer pod virtual server instance has been successfully created.

Create a `terraform.tfvars` file in the [configuration directory](./terraform/run-nginx-demo) for this Terraform configuration on your `development machine`. The `terraform.tfvars` file should look like this

```
ibmcloud_api_key = "<your API Key>"
cluster_name = "<cluster name>"
vpc_name = "<name of your VPC>" OR vpc_id = "<ID of your VPC>"
podvm_image_name = "<name of your peer pod VM image>" OR podvm_image_id = "<ID of your peer pod VM image>"
```

If you created your infrastructure using the other Terraform configurations included in this repo then you can find the ID of your peer pod VM image in the output values of the `podvm-image` Terraform configuration and the ID of your VPC the output values of the `common` Terraform configuration. If you created these resources without using the Terraform configurations then you should find the names or IDs of these resources in your existing IBM Cloud infrastructure and provide them to the respective parameters.

> **Notes:**
> - The `vpc` resource can be specified using its name or ID. The `vpc_name` and `vpc_id` variables are mutually exclusive.
> - Variables with the same name as variables in the end to end Terraform configuration are as described in that [configuration's parameters](#parameters).
> - Additional variables and their defaults are defined in the [variables.tf](./terraform/run-nginx-demo/variables.tf) file for this Terraform configuration.

Execute the following commands on your `development machine` to deploy the nginx demo workload:

```bash
$ cd ibmcloud/terraform/run-nginx-demo
$ terraform init
$ terraform plan
$ terraform apply
```

Deploying the demo workload will create a new configMap, secret, nginx Pod and NodePort service on your Kubernetes cluster. It will also create a new virtual server instance for the peer pod in your IBM Cloud VPC. The `run-nginx-demo` Terraform configuration will also sniff test the deployed nginx server by accessing the HTTP port of the NodePort service, test that the CPU architecture of the Kubernetes worker matches that of the peer pod instance and test the volumes from configMap and secret be mounted correctly.

> **Tip:** You can run the nginx sniff test manually if you log into the Kubernetes worker node using the floating IP that was assigned to it
> ```bash
> $ ssh root@floating-ip-of-worker-node
> ```
> Then run the command
> ```bash
> $ curl http://localhost:30080
> ```
> You can also check the CPU architecture the pod VM instance is using by running the command
> ```bash
> $ kubectl exec nginx -- uname -a
> ```
> While logged into to Kubernetes worker node. If you are using a `s390x` based image as the pod VM image, the output looks like this.
> ```
> Linux nginx 5.4.0-109-generic #123-Ubuntu SMP [Date] s390x GNU/Linux
> ```

> **Note:** The cloud API adaptor establishes a network tunnel between the worker and pod VM, and the network traffic to/from the pod VM is transparently transferred via the tunnel.

> **Tip:** You can also check the status of pod VM instance at [https://cloud.ibm.com/vpc-ext/compute/vs](https://cloud.ibm.com/vpc-ext/compute/vs). Alternatively, you can use the `ibmcloud` command to list your images as follows.
>    ```bash
>    $ ibmcloud is instances
>    ```

> **Tip:** When the peer pod instance is created and it fails to start due to [capacity problems](https://cloud.ibm.com/docs/vpc?topic=vpc-instance-status-messages#cannot-start-capacity).
> Please stop `cloud-api-adaptor` on worker node, try to run peer pod instance on another zone:
> - Create a new subnet on the target zone by hand.
> - Start `cloud-api-adaptor` with new `vpc_zone` and `primary-subnet-id` on worker node.
> - Create nginx demo again. 

If you want to re-run the check, run:
```bash
$ terraform destroy
$ terraform plan
$ terraform apply
```

## Clean up

If you want to clean up the IBM Cloud resources created in the above instructions, you can use the following steps:

### Delete the demo environment end-to-end configuration

To do a full clean up of the demo environment, from your development machine navigate to the `terraform/` repository directory for the end-to-end Terraform configuration with:

```bash
$ cd ibmcloud/terraform
$ terraform destroy
```

This should delete all resources except the peer pod VM custom image, which you can delete by following the [Delete the peer pod VM image](#Delete-the-peer-pod-VM-image) instructions.

### Delete the demo configuration and pod
From your development machine navigate to the `run-nginx-demo` repository directory and delete nginx pod on your Kubernetes cluster with:
```bash
$ cd ibmcloud/terraform/run-nginx-demo
$ terraform destroy
```

If the `cloud-api-adaptor` process was still running `terraform destroy` for this Terraform configuration should automatically delete the peer pod created VM instance too. If the `cloud-api-adaptor` process has stopped, then you can manually check for extra pod VSIs by running:
```bash
$ ibmcloud is instances
```
to see if there are instances other than the control-plane and worker instances.

If so these can be deleted with: 
```bash
$ ibmcloud is instance-delete <peer_pod_instance_id>
```

### Stop the cloud API adaptor process
From your development machine navigate to the `start-cloud-api-adaptor` repository directory and stop the cloud API adaptor process on the Kubernetes worker node with:
```bash
$ cd ibmcloud/terraform/start-cloud-api-adaptor
$ terraform destroy
```

### Delete the peer pod VM image

To check and then delete the custom peer pod VM image created run:
```bash
$ ibmcloud is images --visibility=private
$ ibmcloud is image-delete <image_id>
```

### Delete the cluster

From your development machine navigate to the `cloud-api-adaptor` repository directory and delete the VPC Kubernetes
cluster with:
```bash
$ cd ibmcloud/terraform/cluster
$ terraform destroy
```

### Delete the VPC

From your development machine navigate to the `cloud-api-adaptor` repository directory and delete the VPC, security
groups, subnet and gateway with:
```bash
$ cd ibmcloud/terraform/common
$ terraform destroy
```

## Troubleshooting

Please see the [Troubleshooting Guide](./TROUBLESHOOTING.md), if needed.
