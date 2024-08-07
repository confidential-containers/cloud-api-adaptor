# Cloud API Adaptor (CAA) on AWS

This documentation will walk you through setting up CAA (a.k.a. Peer Pods) on Amazon Elastic Kubernetes Service (EKS). It explains how to deploy:

- One worker EKS
- CAA on that Kubernetes cluster
- An Nginx pod backed by CAA pod VM

> **Note**: Run the following commands from the following directory - `src/cloud-api-adaptor`

## Prerequisites

- Install `aws` CLI [tool](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- Install `eksctl` CLI [tool](https://eksctl.io/installation/)
- Set `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` (or `AWS_PROFILE`) and `AWS_REGION` for AWS cli access
- Install packer by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)
- Install packer's Amazon plugin `packer plugins install github.com/hashicorp/amazon`

## Build CAA pod-VM image

- Set environment variables

```sh
export CLOUD_PROVIDER=aws # mandatory
export AWS_REGION="us-east-1" # mandatory
export PODVM_DISTRO=ubuntu # mandatory
export INSTANCE_TYPE=c4.xlarge # optional, default is c4.xlarge
export IMAGE_NAME=peer-pod-ami # optional
export VPC_ID=REPLACE_ME # optional, otherwise, it creates and uses the default vpc in the specific region
export SUBNET_ID=REPLACE_ME # must be set if VPC_ID is set
```


[Optional] If you want to change the volume size of the generated AMI, then set the `VOLUME_SIZE` environment variable.
For example if you want to set the volume size to 40 GiB, then do the following:

```sh
export VOLUME_SIZE=40
```

[Optional] If you want to use a specific port or address for `agent-protocol-forwarder`, set `FORWARDER_PORT` environment variable.

```sh
export FORWARDER_PORT=<port-number>
```

- Create a custom AWS VM image based on Ubuntu 22.04 having kata-agent and other dependencies

> **NOTE**: For setting up authenticated registry support read this [documentation](../docs/registries-authentication.md).

```sh
cd aws/image
make image
```

You can also build the custom AMI by running the packer build inside a container:

```sh
docker build -t aws \
--secret id=AWS_ACCESS_KEY_ID \
--secret id=AWS_SECRET_ACCESS_KEY \
--build-arg AWS_REGION=${AWS_REGION} \
-f Dockerfile .
```

If you want to use an existing `VPC_ID` with public `SUBNET_ID` then use the following command:

```sh
docker build -t aws \
--secret id=AWS_ACCESS_KEY_ID \
--secret id=AWS_SECRET_ACCESS_KEY \
--build-arg AWS_REGION=${AWS_REGION} \
--build-arg VPC_ID=${VPC_ID} \
--build-arg SUBNET_ID=${SUBNET_ID} \
-f Dockerfile .
```

- Note down your newly created AMI_ID and export it via `PODVM_AMI_ID` env variable

Once the image creation is complete, you can use the following CLI command as well to
get the AMI_ID. The command assumes that you are using the default AMI name: `peer-pod-ami`

```sh
export PODVM_AMI_ID=$(aws ec2 describe-images --query "Images[*].[ImageId]" --filters "Name=name,Values=peer-pod-ami" --region ${AWS_REGION} --output text)
echo ${PODVM_AMI_ID}
```

## Build CAA container image

> **Note**: If you have made changes to the CAA code and you want to deploy those changes then follow [these instructions](../install/README.md#building-custom-cloud-api-adaptor-image) to build the container image from the root of this repository.

If you would like to deploy the latest code from the default branch (`main`) of this repository then expose the following environment variable:

```sh
export registry="quay.io/confidential-containers"
```

## Deploy Kubernetes using EKS

> **Note:**
>
> - Default EKS CNI doesn't work with cloud-api-adaptor (CAA). Ensure you use Calico or Flannel.
> The example given here uses Calico.
> 
> - The commands assumes that the required AWS env variables are set `AWS_REGION`, `AWS_PROFILE` or `AWS_SECRET_ACCESS_KEY` and `AWS_ACCESS_KEY_ID`
>
> - The instructions here have been tested with eksctl version 0.187.0 and Kubernetes 1.30

- Create EKS cluster

Create the cluster without nodegroup.

```sh
EKS_CLUSTER_NAME=my-calico-cluster

eksctl create cluster --name "$EKS_CLUSTER_NAME" --without-nodegroup
```

- Configure Calico networking

```sh
kubectl delete daemonset -n kube-system aws-node

kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.28.0/manifests/tigera-operator.yaml

kubectl create -f - <<EOF
kind: Installation
apiVersion: operator.tigera.io/v1
metadata:
  name: default
spec:
  kubernetesProvider: EKS
  cni:
    type: Calico
  calicoNetwork:
    bgp: Disabled
EOF
```

- Create nodegroup

The following command will create a nodegroup consisting of 2 (default) number of worker nodes.

```sh
eksctl create nodegroup --cluster $EKS_CLUSTER_NAME --node-type m5.xlarge --max-pods-per-node 100 --node-ami-family 'Ubuntu2204'
```

Wait for the cluster to be created.

- Allow required network ports

```sh
EKS_VPC_ID=$(aws eks describe-cluster --name "$EKS_CLUSTER_NAME" \
--query "cluster.resourcesVpcConfig.vpcId" \
--output text)
echo $EKS_VPC_ID

EKS_CLUSTER_SG=$(aws eks describe-cluster --name "$EKS_CLUSTER_NAME" \
   --query "cluster.resourcesVpcConfig.clusterSecurityGroupId" \
   --output text)
echo $EKS_CLUSTER_SG

EKS_VPC_CIDR=$(aws ec2 describe-vpcs --vpc-ids "$EKS_VPC_ID" \
 --query 'Vpcs[0].CidrBlock' --output text)
echo $EKS_VPC_CIDR

# agent-protocol-forwarder port
aws ec2 authorize-security-group-ingress --group-id "$EKS_CLUSTER_SG" --protocol tcp --port 15150 --cidr "$EKS_VPC_CIDR"

#vxlan port
aws ec2 authorize-security-group-ingress --group-id "$EKS_CLUSTER_SG" --protocol tcp --port 9000 --cidr "$EKS_VPC_CIDR"
aws ec2 authorize-security-group-ingress --group-id "$EKS_CLUSTER_SG" --protocol udp --port 9000 --cidr "$EKS_VPC_CIDR"
```

> **Note:**
>
> - Port `15150` is the default port for `cloud-api-adaptor` to connect to the `agent-protocol-forwarder`
> running inside the pod VM.>
> - Port `9000` is the VXLAN port used by `cloud-api-adaptor`. Ensure it doesn't conflict with the VXLAN port
> used by the Kubernetes CNI.

## Deploy CAA

### Create the credentials file

```sh
cat <<EOF > install/overlays/aws/aws-cred.env
AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
EOF
```

### Update the `kustomization.yaml` file

Run the following command to update the [`kustomization.yaml`](../install/overlays/aws/kustomization.yaml) file with the `PODVM_AMI_ID` value:

```bash
sed -i -E "s/(PODVM_AMI_ID=).*/\1 \"${PODVM_AMI_ID}\"/" install/overlays/aws/kustomization.yaml
```

### Deploy CAA on the Kubernetes cluster

Label the cluster nodes with `node.kubernetes.io/worker=`

```sh
for NODE_NAME in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
  kubectl label node $NODE_NAME node.kubernetes.io/worker=
done
```

Run the following command to deploy CAA:

```sh
CLOUD_PROVIDER=aws make deploy
```

Generic CAA deployment instructions are also described [here](../install/README.md).

## Run sample application

### Ensure runtimeclass is present

Verify that the `runtimeclass` is created after deploying CAA:

```sh
kubectl get runtimeclass
```

Once you can find a `runtimeclass` named `kata-remote` then you can be sure that the deployment was successful. A successful deployment will look like this:

```console
$ kubectl get runtimeclass
NAME          HANDLER       AGE
kata-remote   kata-remote   7m18s
```

### Deploy workload

Create an `nginx` deployment:

```yaml
echo '
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx     
    spec:
      runtimeClassName: kata-remote
      containers:
      - image: nginx@sha256:9700d098d545f9d2ee0660dfb155fe64f4447720a0a763a93f2cf08997227279
        name: nginx
' | kubectl apply -f -
```

Ensure that the pod is up and running:

```sh
kubectl get pods -n default
```

You can verify that the peer-pod VM was created by running the following command:

```sh
aws ec2 describe-instances --filters "Name=tag:Name,Values=podvm*" \
   --query 'Reservations[*].Instances[*].[InstanceId, Tags[?Key==`Name`].Value | [0]]' --output table
```

Here you should see the VM associated with the pod `nginx`. If you run into problems then check the troubleshooting guide [here](../docs/troubleshooting/README.md).

## Cleanup

Delete all running pods using the runtimeClass `kata-remote`. You can use the following command for the same:

```sh
kubectl get pods -A -o custom-columns='NAME:.metadata.name,NAMESPACE:.metadata.namespace,RUNTIMECLASS:.spec.runtimeClassName' | grep kata-remote | awk '{print $1, $2}'
```

Verify that all peer-pod VMs are deleted. You can use the following command to list all the peer-pod VMs
(VMs having prefix `podvm`) and status:

```sh
aws ec2 describe-instances --filters "Name=tag:Name,Values=podvm*" \
--query 'Reservations[*].Instances[*].[InstanceId, Tags[?Key==`Name`].Value | [0], State.Name]' --output table
```

Delete the EKS cluster by running the following command:

```sh
EKS_NODEGROUP=$(eksctl get nodegroup --cluster "$EKS_CLUSTER_NAME" -o json | jq -r ".[].Name")
echo $EKS_NODEGROUP

eksctl delete nodegroup --cluster=$EKS_CLUSTER_NAME --name=$EKS_NODEGROUP --disable-eviction --drain=false --timeout=10m 

eksctl delete cluster --name=$EKS_CLUSTER_NAME
```
