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

## Create pod VM AMI

Building a pod VM AMI is a two step process. First you will need to create a raw image and then
use the raw image to create the AMI

1. Create pod VM raw image.

    The pod VM raw image is created using `mkosi`. Refer to the [README](../podvm-mkosi/README.md) to
    build the raw image.

    An example invocation to build the raw image with support for `snp` TEE:

    ```sh
    cd podvm-mkosi
    export CLOUD_PROVIDER=aws
    export TEE_PLATFORM=snp
    make image
    ```

    The built image will be available in the following path:  `build/system.raw`

1. Create the AMI.

    You can use `uplosi` as described in the [README](../podvm-mkosi/README.md) or
    you can use the `raw-to-ami.sh` script to upload the raw image to AWS S3 bucket and create the AMI.

    Set the `PODVM_AMI_ID` env variable with the AMI id.

    ```sh
    export PODVM_AMI_ID=<AMI_ID_CREATED>
    ```

    > **Note:**
    > There is a pre-built AMI ID `ami-0c2afc4cc79cb9083` in `us-east-2` that you can use for testing.

## Deploy Kubernetes using EKS

1. Create EKS cluster.

    Example EKS cluster creation using the default AWS VPC-CNI

    ```sh
    export EKS_CLUSTER_NAME=<set_cluster_name>
    eksctl create cluster --name "$EKS_CLUSTER_NAME" \
        --node-type m5.xlarge \
        --node-ami-family Ubuntu2204 \
        --nodes 1 \
        --nodes-min 0 \
        --nodes-max 2 \
        --node-private-networking
    ```

    Wait for the cluster to be created.

    > **Note:**
    > If you are using Calico CNI, then you'll need to run the webhook using `hostNetwork: true`. See
    > the following [issue](https://github.com/confidential-containers/cloud-api-adaptor/issues/2138) for more details.

1. Allow required network ports.

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

At a minimum you need to update `PODVM_AMI_ID` and `VXLAN_PORT` values
in [`kustomization.yaml`](../install/overlays/aws/kustomization.yaml).

```sh
sed -i -E "s/(PODVM_AMI_ID=).*/\1 \"${PODVM_AMI_ID}\"/" install/overlays/aws/kustomization.yaml
sed -i 's/^\([[:space:]]*\)#- VXLAN_PORT=.*/\1- VXLAN_PORT=9000/'  install/overlays/aws/kustomization.yaml
```

Have a look at other parameters and update it if required.

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

Here you should see the VM associated with the pod `nginx`. 
If you run into problems then check the troubleshooting guide [here](../docs/troubleshooting/README.md).

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
eksctl delete cluster --name=$EKS_CLUSTER_NAME 
```
