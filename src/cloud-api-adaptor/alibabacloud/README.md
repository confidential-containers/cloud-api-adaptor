# Cloud API Adaptor (CAA) on Alibaba Cloud

This documentation will walk you through setting up CAA (a.k.a. Peer Pods) on Alibaba Cloud Container Service for Kubernetes (ACK) and Alibaba Cloud Elastic Compute Service (ECS). It explains how to deploy:

- One worker for ACK Managed Cluster
- CAA on that Kubernetes cluster
- An Nginx pod backed by CAA pod VM on ECS

> **Note**: Run the following commands from the following directory - `src/cloud-api-adaptor`

> **Note**: Now Confidential Computing instances [are only available](https://www.alibabacloud.com/help/en/ecs/user-guide/build-a-tdx-confidential-computing-environment) in `cn-beijing` region within zone `cn-beijing-i`. 

## Prerequisites

- Install `aliyun` CLI [tool](https://www.alibabacloud.com/help/en/cli/installation-guide/?spm=a2c63.p38356.help-menu-29991.d_2.28f346a6IMqkop) and [configure credentials](https://www.alibabacloud.com/help/en/cli/configure-credentials)
- Have an `aliyun` OSS storage with a bucket.

## Create pod VM Image

> **Note:**
> There is a pre-built Community Image (id:`m-2ze1w9aj2aonwckv64cw`) for version `0.13.0` in `cn-beijing` that you can use for testing.

If you want to build a pod VM image yourself, please follow the steps.

1. Create pod VM image.

    ```sh
    PODVM_DISTRO=alinux \
    CLOUD_PROVIDER=alibabacloud \
    IMAGE_URL=https://alinux3.oss-cn-hangzhou.aliyuncs.com/aliyun_3_x64_20G_nocloud_alibase_20250117.qcow2 \
    make podvm-builder podvm-binaries podvm-image
    ```

    The built image will be available in the root path of following newly built docker image: `quay.io/confidential-containers/podvm-alibabacloud-alinux-amd64:<sha256>`
    with name like `podvm-*.qcow2`. You need to export it from the container image.

2. Upload to OSS storage and create ECS Image.

    You will then need to upload the Pod VM image to OSS (Object Storage Service). 
    ```sh
    export REGION_ID=<region-id>
    export IMAGE_FILE=<path-to-qcow2-file>
    export BUCKET=<OSS-bucket-name>
    export OBJECT=<object-name>

    aliyun oss cp ${IMAGE_FILE} oss://${BUCKET}/${OBJECT}
    ```

    Then, mark the image file as an ECS Image
    ```sh
    export IMAGE_NAME=$(basename ${IMAGE_FILE%.*})

    aliyun ecs ImportImage --ImageName ${IMAGE_NAME} \
        --region ${REGION_ID} --RegionId ${REGION_ID}
        --BootMode UEFI \
        --DiskDeviceMapping.1.OSSBucket ${BUCKET} --DiskDeviceMapping.1.OSSObject ${OBJECT} \
        --Features.NvmeSupport supported \
        --method POST --force
    
    export POD_IMAGE_ID=<ImageId>
    ```

## Build CAA development image

If you want to build CAA daemonset image yourself:

  ```sh
  export registry=<registry-address>
  export RELEASE_BUILD=true
  export CLOUD_PROVIDER=alibabacloud
  make image
  ```

After that you should take note of the tag used for this image, we will use it
later.

## Deploy Kubernetes using ACK Managed Cluster

1. Create ACK Managed Cluster.

    ```sh
    export CONTAINER_CIDR=172.18.0.0/16
    export REGION_ID=cn-beijing
    export ZONES='["cn-beijing-i"]'

    aliyun cs CreateCluster --header "Content-Type=application/json" --body "
    {
      \"cluster_type\":\"ManagedKubernetes\",
      \"name\":\"caa\",
      \"region_id\":\"${REGION_ID}\",
      \"zone_ids\":${ZONES},
      \"enable_rrsa\":true,
      \"container_cidr\":\"${CONTAINER_CIDR}\",
      \"addons\":[
        {
          \"name\":\"flannel\"
        }
      ]
    }"
    
    export CLUSTER_ID=<cluster-id>
    export SECURITY_GROUP_ID=$(aliyun cs DescribeClusterDetail --ClusterId ${CLUSTER_ID} | jq -r  ".security_group_id")
    ```

    Wait for the cluster to be created. Get the vSwitch id of the cluster.
    
    ```sh
    VSWITCH_IDS=$(aliyun cs DescribeClusterDetail --ClusterId ${CLUSTER_ID} | jq -r  ".parameters.WorkerVSwitchIds" | sed 's/^/["/; s/$/"]/; s/,/","/g')
    ```
    Then add one worker node to the cluster.

    ```sh
    WORKER_NODE_COUNT=1
    WORKER_NODE_TYPE="[\"ecs.g8i.xlarge\",\"ecs.g7.xlarge\"]"
    aliyun cs POST /clusters/${CLUSTER_ID}/nodepools \
      --region ${REGION_ID} \
      --header "Content-Type=application/json;" \
      --body "
    {
      \"nodepool_info\": {
        \"name\":\"worker-node-pool\"
      },
      \"management\":{
        \"enable\":true
      },
      \"scaling_group\":{
        \"desired_size\":${WORKER_NODE_COUNT},
        \"image_type\":\"Ubuntu\",
        \"instance_types\":${WORKER_NODE_TYPE},
        \"system_disk_category\":\"cloud_essd\",
        \"vswitch_ids\":${VSWITCH_IDS},
        \"system_disk_size\":40,
        \"internet_charge_type\":\"PayByTraffic\",
        \"internet_max_bandwidth_out\":5,
        \"data_disks\":[
          {
            \"size\":120,
            \"category\":\"cloud_essd\",
            \"system_disk_bursting_enabled\":false
          }
        ]
      }
    }"
    
    NODE_POOL_ID=<node-pool-id>
    ```

2. Add Internet access for the cluster VPC

    ```sh
    export VPC_ID=$(aliyun cs DescribeClusterDetail --ClusterId ${CLUSTER_ID} | jq -r ".vpc_id")
    export VSWITCH_ID=$(echo ${VSWITCH_IDS} | sed 's/[][]//g' | sed 's/"//g')
    aliyun vpc CreateNatGateway \
      --region ${REGION_ID} \
      --RegionId ${REGION_ID} \
      --VpcId ${VPC_ID} \
      --NatType Enhanced \
      --VSwitchId ${VSWITCH_ID} \
      --NetworkType internet
    
    export GATEWAY_ID="<NatGatewayId>"
    export SNAT_TABLE_ID="<SnatTableId>"

    # The band width of the public ip (Mbps)
    export BAND_WIDTH=5
    aliyun vpc AllocateEipAddress \
      --region ${REGION_ID} \
      --RegionId ${REGION_ID} \
      --Bandwidth ${BAND_WIDTH}

    export EIP_ID="<AllocationId>"
    export EIP_ADDRESS="<EipAddress>"
  
    aliyun vpc AssociateEipAddress \
      --region ${REGION_ID} \
      --RegionId ${REGION_ID} \
      --AllocationId ${EIP_ID} \
      --InstanceId ${GATEWAY_ID} \
      --InstanceType Nat
    
    aliyun vpc CreateSnatEntry \
      --region ${REGION_ID} \
      --RegionId ${REGION_ID} \
      --SnatTableId ${SNAT_TABLE_ID} \
      --SourceVSwitchId ${VSWITCH_ID} \
      --SnatIp ${EIP_ADDRESS}
    ```

3. Grant role permissions

    Give role permission to the cluster to allow the worker to create ECS instances.
    ```sh
    export ROLE_NAME=caa-alibaba
    export RRSA_ISSUER=$(aliyun cs DescribeClusterDetail --ClusterId ${CLUSTER_ID} | jq -r ".rrsa_config.issuer" | cut -d',' -f1)
    export RRSA_ARN=$(aliyun cs DescribeClusterDetail --ClusterId ${CLUSTER_ID} | jq -r ".rrsa_config.oidc_arn" | cut -d',' -f1)
  
    aliyun ram CreateRole --region ${REGION_ID} \
      --RoleName ${ROLE_NAME} \
      --AssumeRolePolicyDocument "
    {
      \"Version\": \"1\",
      \"Statement\": [
        {
          \"Action\": \"sts:AssumeRole\",
          \"Condition\": {
            \"StringEquals\": {
              \"oidc:aud\": [
                \"sts.aliyuncs.com\"
              ],
              \"oidc:iss\": [
                \"${RRSA_ISSUER}\"
              ],
              \"oidc:sub\": [
                \"system:serviceaccount:confidential-containers-system:cloud-api-adaptor\"
              ]
            }
          },
          \"Effect\": \"Allow\",
          \"Principal\": {
            \"Federated\": [
              \"${RRSA_ARN}\"
            ]
          }
        }
      ]
    }"

    export POLICY_NAME=caa-aliyun-policy
    aliyun ram CreatePolicy --region ${REGION_ID} \
      --PolicyName ${POLICY_NAME} \
      --PolicyDocument "
      {
        \"Version\": \"1\",
        \"Statement\": [
          {
            \"Effect\": \"Allow\",
            \"Action\": [
              \"ecs:RunInstances\",
              \"ecs:DeleteInstance\",
              \"ecs:DescribeInstanceAttribute\",
              \"ecs:CreateNetworkInterface\",
              \"ecs:DeleteNetworkInterface\",
              \"ecs:AttachNetworkInterface\",
              \"ecs:ModifyNetworkInterfaceAttribute\",
              \"ecs:DescribeNetworkInterfaceAttribute\"
            ],
            \"Resource\": \"*\"
          },
          {
            \"Effect\": \"Allow\",
            \"Action\": \"vpc:DescribeVSwitchAttributes\",
            \"Resource\": \"*\"
          },
          {
            \"Effect\": \"Allow\",
            \"Action\": [
              \"vpc:AllocateEipAddress\",
              \"vpc:ReleaseEipAddress\",
              \"vpc:AssociateEipAddress\",
              \"vpc:UnassociateEipAddress\",
              \"vpc:DescribeEipAddresses\",
              \"vpc:DescribeVSwitchAttributes\"
            ],
            \"Resource\": \"*\"
          }
        ]
      }
      "

    aliyun ram AttachPolicyToRole \
      --region ${REGION_ID} \
      --PolicyType Custom \
      --PolicyName ${POLICY_NAME} \
      --RoleName ${ROLE_NAME}

    ROLE_ARN=$(aliyun ram GetRole --region ${REGION_ID} --RoleName ${ROLE_NAME} | jq -r ".Role.Arn")
    ```

## Deploy CAA

### Create the credentials file

```sh
cat <<EOF > install/overlays/alibabacloud/alibabacloud-cred.env
# If the WorkerNode is on ACK, we use RRSA to authenticate
ALIBABA_CLOUD_ROLE_ARN=${ROLE_ARN}
ALIBABA_CLOUD_OIDC_PROVIDER_ARN=${RRSA_ARN}
ALIBABA_CLOUD_OIDC_TOKEN_FILE=/var/run/secrets/ack.alibabacloud.com/rrsa-tokens/token
EOF
```

### Update the `kustomization.yaml` file

At a minimum you need to update the followint values
in [`kustomization.yaml`](../install/overlays/alibabacloud/kustomization.yaml).

- `VSWITCH_ID`: Use one of the values of `${VSWITCH_IDS}`
- `SECURITY_GROUP_IDS`: We can reuse the ACK's security group id `${SECURITY_GROUP_ID}`.
- `IMAGEID`: The ECS images ID, e.g. `m-2ze1w9aj2aonwckv64cw` in `cn-beijing` region.
- `REGION`: The region where Peer Pods run, e.g. `cn-beijing`. 

### Deploy CAA on the Kubernetes cluster

Label the cluster nodes with `node.kubernetes.io/worker=`

```sh
for NODE_NAME in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
  kubectl label node $NODE_NAME node.kubernetes.io/worker=
done
```

Run the following command to deploy CAA:

> **Note**: We have a [forked version](https://github.com/AliyunContainerService/coco-operator)
of CoCo Operator for Alibaba Cloud. Specifically,
we enabled containerd 1.7+ installation and mirrored images from `quay.io` on
Alibaba Cloud to accelerate.

```sh
export COCO_OPERATOR_REPO="https://github.com/AliyunContainerService/coco-operator"
export COCO_OPERATOR_REF="main"
export RESOURCE_CTRL=false
export CLOUD_PROVIDER=alibabacloud
make deploy
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
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  runtimeClassName: kata-remote
  containers:
  - name: nginx
    image: registry.openanolis.cn/openanolis/nginx:1.14.1-8.6
' | kubectl apply -f -
```

Ensure that the pod is up and running:

```sh
kubectl get pods -n default
```

You can verify that the peer-pod VM was created by running the following command:

```sh
aliyun ecs DescribeInstances --RegionId ${REGION_ID} --InstanceName 'podvm-*'
```

Here you should see the VM associated with the pod `nginx`. 
If you run into problems then check the troubleshooting guide [here](../docs/troubleshooting/README.md).

## Attestation

TODO

## Cleanup

Delete all running pods using the runtimeClass `kata-remote`.

Verify that all peer-pod VMs are deleted. You can use the following command to list all the peer-pod VMs
(VMs having prefix `podvm`) and status:

```sh
aliyun ecs DescribeInstances --RegionId ${REGION_ID} --InstanceName 'podvm-*'
```

Delete the ACK cluster by running the following command:

```sh
aliyun cs DELETE /clusters/${CLUSTER_ID} --region ${REGION_ID} --keep_slb false --retain_all_resources false --header "Content-Type=application/json;" --body "{}"
```
