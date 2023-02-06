# Setup procedure for an IBM Cloud self-managed cluster

This Terraform script creates an *n*-node self-managed Kubernetes cluster using ibmcloud infrastructure.
This can then be used to test the cloud-api adaptor and peer-pods.

## Prerequisites

To create the Kubernetes cluster, you need to install terraform, Ansible and the IBM Cloud CLI on your 
'development machine'. To manage the cluster you will need to install `kubectl`.
Please follow the the official installation guides:

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

* [Installing the stand-alone IBM Cloud CLI](https://cloud.ibm.com/docs/cli?topic=cli-install-ibmcloud-cli)

> **Tips**
> - If you are using Ubuntu linux, you can run follow commands simply:
>     ```bash
>     $ curl -fsSL https://clis.cloud.ibm.com/install/linux | sh
>     $ ibmcloud plugin install vpc-infrastructure
>     $ ibmcloud plugin install cloud-object-storage
>     ```
> - You can use the [IBM Cloud Web UI](https://cloud.ibm.com/vpc-ext/overview) for most of the operations of IBM Cloud.
And please make sure that you are selecting the correct region in the Web UI.
> 

* You need IBM Cloud API key. You can create your own API key at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).

* [Install `kubectl` to run commands against the Kubernetes cluster](https://kubernetes.io/docs/tasks/tools/)

## Create an IBM Cloud Virtual Private Cloud (VPC) and 'self-managed' Kubernetes Cluster -

The Terraform configuration supports building the cluster on both the x86 (Intel) and s390x (IBM Z) architectures.

To use the Terraform configuration, you need to create a file `terraform.tfvars` to specify parameters for the
Terraform configuration. The `terraform.tfvars` file with all mandatory parameters looks like this:

```
ibmcloud_api_key = "<your API key>"
ssh_key_name = "<name of your IBM Cloud create SSH key>"
```

There are also a number of optional fields:
```
cluster_name = "<name of your cluster>
region_name = "<name of an IBM Cloud region>"
zone_name = "<name of a zone in your IBM Cloud zone region>"
ssh_pub_key = "<your SSH public key - this results in the ssh_key being created on IBM Cloud>"
node_image = "<name of the image to use for the Kubernetes nodes"
node_profile = "<name of the instance profile to use for the Kubernetes nodes>"
nodes = <number of nodes created in the cluster>
containerd_version = "<the version of containerd installed on the Kubernetes nodes>"
```

#### Parameters

> **Notes:**
> - `ibmcloud_api_key` is your IBM Cloud API key that you created at 
[https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).
> - `ssh_key_name` is the name of your SSH key registered in IBM Cloud or the name of a new SSH key if a public key is
also provided using the optional `ssh_pub_key` parameter. You can add your SSH key at 
[https://cloud.ibm.com/vpc-ext/compute/sshKeys](https://cloud.ibm.com/vpc-ext/compute/sshKeys). For more information
about SSH keys see [managing SSH Keys](https://cloud.ibm.com/docs/vpc?topic=vpc-ssh-keys). The SSH key will be 
installed on the Kubernetes control plane and worker nodes and is used to access them from your 'development machine' and for terraform to perform the cluster set up.
> - `cluster_name` (optional) is a name of a Kubernetes cluster. This name is used for the prefix of the names of 
Kubernetes node virtual server instances, the VPC and the subnet. If not set it defaults to `caa-cluster`.
> - `region_name` (optional) is the IBM Cloud region Terraform will create the demo environment in. If not set it
defaults to `jp-tok`.
> - `zone_name` (optional) is the zone in the region Terraform will create the demo environment in. If not set it
defaults to `jp-tok-2`.
> - `ssh_pub_key` (optional) is an variable for a SSH public key which has **not** been registered in IBM Cloud in the
targeted region. Terraform will manage this key instead. You cannot register the same SSH public key in the same region
twice under different SSHs key names. This key needs to be password-less and on the 'developer machine' running the terraform in order to perform the cluster set up.
> - `node_image` (optional) is a name of IBM Cloud infrastructure image. This name is used to create virtual server
 instances for the Kubernetes control plane and worker. For more information, about VPC custom images, see 
 [IBM Cloud Importing and managing custom images](https://cloud.ibm.com/docs/vpc?topic=vpc-managing-images).
  If not set it defaults to `ibm-ubuntu-20-04-2-minimal-s390x-1` for **`s390x`** architecture.
> - `node_profile` (optional) is a name of IBM Cloud virtual server instance profile. This name is used to create virtual server instances for the Kubernetes control plane and worker. For more information, about virtual server instance profile, see [instance profiles](https://cloud.ibm.com/docs/vpc?topic=vpc-profiles). If not set it defaults to `bz2-2x8`, which uses the `s390x` architecture, has 2 vCPUs and 8 GB memory.
> - `nodes` (optional) is the number of VSIs created to be used as Kubernetes nodes. There will be a single control 
plane node and the rest will be worker nodes. If not set it defaults to `2`.
> - `containerd_version` (optional) is the version of containerd installed on the Kubernetes nodes. If not set it 
defaults to `1.7.0-beta.3`.
 <!-- TODO #570 once we've fixed pre-install of containerd note that this might be overriden?-->

> **Hint:** In order to create a cluster based on a different type of VSI image you can use the `instance_profile_name`
and `image_name` parameters. E.g., to create an **x86** architecture based cluster, include the following two lines in
the `terraform.tfvars` file
>
>     node_profile = "bx2-2x8"
>     node_image = "ibm-ubuntu-20-04-3-minimal-amd64-1"
>

After writing you `terraform.tfvars` file you can create your VPC by executing the following commands on your
'development machine`':
```bash
$ terraform init
$ terraform plan
$ terraform apply
```

The following IBM Cloud resources will be created when running the end-to-end Terraform configuration:
* VPC
* Security groups
* Subnets
* Public gateway
* Floating IP for the public gateway
* Virtual server instances for the Kubernetes nodes
* Floating IPs for the Kubernetes control plane and worker virtual server instances
* SSH key, if you specified the optional `ssh_pub_key` variable

## Connect to the cluster and test it

Once the terraform process has completed, the IBM Cloud VPC resources will have been created and it will have created
a file `config` which is the
[kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
for the cluster. 

To create a quick nginx pod to test the cluster, you can run the follow steps:
- From within the `cloud-api-adaptor/ibmcloud/cluster` directory, run `export KUBECONFIG="$(pwd)/config"` to let the
system know to use the `config` file for Kubernetes. Alternatively add the `--kubeconfig config` option to all
`kubectl` commands.
- Create an nginx deployment with:
```
$ kubectl create -f https://k8s.io/examples/application/deployment.yaml
```
- Check the pods were created successfully with:
```
$ kubectl get pods
```
This should result in something like:
```
NAME                                READY   STATUS    RESTARTS   AGE
nginx-deployment-85996f8dbd-26kls   1/1     Running   0          8s
nginx-deployment-85996f8dbd-m555l   1/1     Running   0          8s
```
- The deployment can be removed by running:
```
$ kubectl delete -f https://k8s.io/examples/application/deployment.yaml
```

## Delete the cluster

Once you are finished with the cluster and ready to free up the IBM Cloud resources, from your 'development machine'
navigate to the `cloud-api-adaptor/ibmcloud/cluster` repository directory and delete the VPC Kubernetes cluster with:
```bash
$ terraform destroy
```
