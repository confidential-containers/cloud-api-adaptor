# AWS IRSA Setup Guide for Cloud API Adaptor

This guide explains how to configure and use IAM Roles for Service Accounts (IRSA) with Cloud API Adaptor on Amazon EKS.

## Overview

IRSA allows your CAA pods to assume an IAM role using OpenID Connect (OIDC), eliminating the need for static AWS credentials stored in Kubernetes secrets. This is the recommended approach for AWS authentication on EKS.

> **EKS-Specific Implementation**
>
> This guide is designed for **Amazon EKS** clusters, which automatically inject IRSA environment variables (`AWS_WEB_IDENTITY_TOKEN_FILE`, `AWS_ROLE_ARN`) and mount service account tokens when you annotate a ServiceAccount.
>
> **For self-managed Kubernetes or other platforms:**
> - You must manually configure projected service account token volumes
> - Set `AWS_WEB_IDENTITY_TOKEN_FILE` and `AWS_ROLE_ARN` environment variables in the ConfigMap/Secret
> - Mount the OIDC token at the path specified in `AWS_WEB_IDENTITY_TOKEN_FILE`
> - See the [Alibaba Cloud RRSA implementation](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/src/cloud-api-adaptor/install/charts/peerpods/templates/daemonset.yaml#L69-L128) in the Helm charts as a reference for manual OIDC token projection

## Benefits

- **No static credentials** - No long-lived AWS access keys in Kubernetes secrets
- **Automatic credential rotation** - Temporary credentials that auto-rotate
- **Fine-grained permissions** - Each service account can have different IAM roles
- **AWS best practice** - Follows AWS security recommendations for EKS workloads

## Prerequisites

- EKS cluster version 1.13 or later
- `kubectl` configured to access your cluster
- `aws` CLI version 2.0 or later
- `eksctl`
- Appropriate AWS IAM permissions to create roles and OIDC providers

## Step 1: Enable OIDC Provider for Your EKS Cluster

### Check if OIDC Issuer Exists

```bash
# Set your cluster name
export CLUSTER_NAME="my-eks-cluster"
export AWS_REGION="us-east-2"

# Get the OIDC issuer URL
aws eks describe-cluster \
  --name ${CLUSTER_NAME} \
  --region ${AWS_REGION} \
  --query "cluster.identity.oidc.issuer" \
  --output text
```

This should output something like:
```
https://oidc.eks.us-east-2.amazonaws.com/id/31D259E489F3F8CDF111109DB7105368
```

### Check if IAM OIDC Provider is Registered

```bash
# Extract the OIDC ID
OIDC_ID=$(aws eks describe-cluster \
  --name ${CLUSTER_NAME} \
  --region ${AWS_REGION} \
  --query "cluster.identity.oidc.issuer" \
  --output text | awk -F'/' '{print $NF}')

# Check if provider exists in IAM
aws iam list-open-id-connect-providers | grep ${OIDC_ID}
```

**If the command returns empty**, the OIDC provider is not registered in IAM and must be created.

#### Create OIDC Provider (if needed)

```bash
eksctl utils associate-iam-oidc-provider \
  --cluster ${CLUSTER_NAME} \
  --region ${AWS_REGION} \
  --approve

# Verify the OIDC provider was created successfully
aws iam list-open-id-connect-providers | grep ${OIDC_ID}
```

### Get Account ID and OIDC Provider

These values will be used for both cloud-api-adaptor and peerpod-ctrl:

```bash
export ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

export OIDC_PROVIDER=$(aws eks describe-cluster \
  --name ${CLUSTER_NAME} \
  --region ${AWS_REGION} \
  --query "cluster.identity.oidc.issuer" \
  --output text | sed 's|https://||')

echo "Account ID: ${ACCOUNT_ID}"
echo "OIDC Provider: ${OIDC_PROVIDER}"
```

## Step 2: Create IAM Role and Attach Permissions for cloud-api-adaptor

### Define Variables

```bash
export NAMESPACE="confidential-containers-system"
export CAA_SERVICE_ACCOUNT="cloud-api-adaptor"
export CAA_ROLE_NAME="CAA-IRSA-Role"
export CAA_POLICY_NAME="CAA-EC2-Policy"
```

### Create Trust Policy for cloud-api-adaptor

```bash
cat > /tmp/caa-trust-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::${ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "${OIDC_PROVIDER}:sub": "system:serviceaccount:${NAMESPACE}:${CAA_SERVICE_ACCOUNT}",
          "${OIDC_PROVIDER}:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
EOF
```

### Create IAM Role for cloud-api-adaptor

```bash
aws iam create-role \
  --role-name ${CAA_ROLE_NAME} \
  --assume-role-policy-document file:///tmp/caa-trust-policy.json \
  --description "IRSA role for Cloud API Adaptor on EKS"
```

### Create and Attach EC2 Permissions Policy

Choose one of the following options:

**Option A: Use AWS Managed Policy (Simpler)**

```bash
aws iam attach-role-policy \
  --role-name ${CAA_ROLE_NAME} \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2FullAccess
```

**Option B: Create Custom Policy (Least Privilege)**

```bash
# Create custom policy
cat > /tmp/caa-ec2-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PeerPodInstanceManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:RunInstances",
        "ec2:TerminateInstances",
        "ec2:DescribeInstances",
        "ec2:DescribeInstanceTypes",
        "ec2:DescribeImages",
        "ec2:CreateNetworkInterface",
        "ec2:AttachNetworkInterface",
        "ec2:DeleteNetworkInterface",
        "ec2:ModifyNetworkInterfaceAttribute",
        "ec2:AllocateAddress",
        "ec2:AssociateAddress",
        "ec2:DisassociateAddress",
        "ec2:ReleaseAddress",
        "ec2:DescribeAddresses",
        "ec2:CreateTags",
        "ec2:DescribeSubnets",
        "ec2:DescribeSecurityGroups"
      ],
      "Resource": "*"
    }
  ]
}
EOF

# Create the policy
aws iam create-policy \
  --policy-name ${CAA_POLICY_NAME} \
  --policy-document file:///tmp/caa-ec2-policy.json

# Attach the policy
aws iam attach-role-policy \
  --role-name ${CAA_ROLE_NAME} \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/${CAA_POLICY_NAME}
```

### Get cloud-api-adaptor Role ARN

```bash
export CAA_ROLE_ARN=$(aws iam get-role \
  --role-name ${CAA_ROLE_NAME} \
  --query 'Role.Arn' \
  --output text)

echo "CAA Role ARN: ${CAA_ROLE_ARN}"
```

Save this ARN - you'll need it for deployment!

## Step 3: Create IAM Role and Attach Permissions for peerpod-ctrl

**Note:** This step is only required if you are deploying peerpod-ctrl. If peerpod-ctrl is not enabled in your deployment (`resourceCtrl.enabled: false`), you can skip this step.

### Define Variables for peerpod-ctrl

```bash
# Note: ACCOUNT_ID, OIDC_PROVIDER, and NAMESPACE were set in Step 1
export CTRL_SERVICE_ACCOUNT="peerpodctrl-controller-manager"
export CTRL_ROLE_NAME="PeerpodCtrl-IRSA-Role"
export CTRL_POLICY_NAME="PeerpodCtrl-EC2-Policy"
```

### Create Trust Policy for peerpod-ctrl

```bash
cat > /tmp/peerpod-ctrl-trust-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::${ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "${OIDC_PROVIDER}:sub": "system:serviceaccount:${NAMESPACE}:${CTRL_SERVICE_ACCOUNT}",
          "${OIDC_PROVIDER}:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
EOF
```

### Create IAM Role for peerpod-ctrl

```bash
aws iam create-role \
  --role-name ${CTRL_ROLE_NAME} \
  --assume-role-policy-document file:///tmp/peerpod-ctrl-trust-policy.json \
  --description "IRSA role for PeerPod Controller on EKS"
```

### Create and Attach EC2 Permissions Policy for peerpod-ctrl

Choose one of the following options:

**Option A: Use AWS Managed Policy (Simpler)**

```bash
aws iam attach-role-policy \
  --role-name ${CTRL_ROLE_NAME} \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2FullAccess
```

**Option B: Create Custom Policy (Least Privilege - Cleanup Only)**

```bash
# Create custom policy for cleanup operations
cat > /tmp/peerpod-ctrl-ec2-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PeerPodCleanup",
      "Effect": "Allow",
      "Action": [
        "ec2:TerminateInstances",
        "ec2:DescribeInstances",
        "ec2:DescribeAddresses",
        "ec2:DisassociateAddress",
        "ec2:ReleaseAddress",
        "ec2:DeleteNetworkInterface"
      ],
      "Resource": "*"
    }
  ]
}
EOF

# Create the policy
aws iam create-policy \
  --policy-name ${CTRL_POLICY_NAME} \
  --policy-document file:///tmp/peerpod-ctrl-ec2-policy.json

# Attach the policy
aws iam attach-role-policy \
  --role-name ${CTRL_ROLE_NAME} \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/${CTRL_POLICY_NAME}
```

### Get peerpod-ctrl Role ARN

```bash
export CTRL_ROLE_ARN=$(aws iam get-role \
  --role-name ${CTRL_ROLE_NAME} \
  --query 'Role.Arn' \
  --output text)

echo "Controller Role ARN: ${CTRL_ROLE_ARN}"
```

Save this ARN - you'll need it for deployment!

## Step 4: Deploy cloud-api-adaptor and peerpod-ctrl with IRSA

### Option A: Deploy Using Helm with --set Flags

```bash
# Deploy both cloud-api-adaptor and peerpod-ctrl with IRSA
# Note: peerpod-ctrl is a subchart (resourceCtrl) of the main peerpods chart
helm install peerpods ./install/charts/peerpods \
  --namespace confidential-containers-system \
  --create-namespace \
  --set provider=aws \
  --set "daemonset.serviceAccount.annotations.eks\.amazonaws\.com/role-arn=${CAA_ROLE_ARN}" \
  --set "resourceCtrl.serviceAccount.annotations.eks\.amazonaws\.com/role-arn=${CTRL_ROLE_ARN}" \
  --set providerConfigs.aws.PODVM_AMI_ID=ami-0123456789abcdef0 \
  --set providerConfigs.aws.AWS_REGION=${AWS_REGION} \
  --set providerConfigs.aws.AWS_SUBNET_ID=subnet-0123456789abcdef0 \
  --set providerConfigs.aws.AWS_SG_IDS=sg-0123456789abcdef0 \
  --set providerConfigs.aws.PODVM_INSTANCE_TYPE=m6a.large
```

**Note:**
- Replace the AMI ID, subnet ID, and security group IDs with your actual values.
- Do NOT set `AWS_ACCESS_KEY_ID` or `AWS_SECRET_ACCESS_KEY` when using IRSA.
- If not deploying peerpod-ctrl, omit the `resourceCtrl.serviceAccount.annotations` line or set `--set resourceCtrl.enabled=false`.
- **Non-EKS environments**: You must also add `AWS_WEB_IDENTITY_TOKEN_FILE` and `AWS_ROLE_ARN` to `providerConfigs.aws`, configure projected token volumes, and mount them into the pod. See the warning at the top of this guide.

### Option B: Deploy Using Helm with values file

Set the annotations in your values file:

```yaml
provider: aws

# Configure IRSA for cloud-api-adaptor
daemonset:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/CAA-IRSA-Role  # Your CAA_ROLE_ARN
...
# Configure IRSA for peerpod-ctrl (optional - only if deploying peerpod-ctrl)
resourceCtrl:
  enabled: true  # Set to false if not using peerpod-ctrl
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/PeerpodCtrl-IRSA-Role  # Your CTRL_ROLE_ARN
...
```

Deploy:

```bash
helm install peerpods ./install/charts/peerpods \
  --namespace confidential-containers-system \
  --create-namespace \
  -f aws.yaml
```

### Option C: Add IRSA to Existing Deployment

If you already have components deployed with static credentials:

```bash
# Annotate cloud-api-adaptor service account
kubectl annotate serviceaccount cloud-api-adaptor \
  -n confidential-containers-system \
  eks.amazonaws.com/role-arn=${CAA_ROLE_ARN} \
  --overwrite

# Restart cloud-api-adaptor
kubectl rollout restart daemonset/cloud-api-adaptor-daemonset \
  -n confidential-containers-system

# If using peerpod-ctrl, annotate its service account and restart
kubectl annotate serviceaccount peerpodctrl-controller-manager \
  -n confidential-containers-system \
  eks.amazonaws.com/role-arn=${CTRL_ROLE_ARN} \
  --overwrite

kubectl rollout restart deployment/peerpodctrl-controller-manager \
  -n confidential-containers-system

# After verifying IRSA is working (see Step 5), remove the static credentials secret
kubectl delete secret peer-pods-secret \
  -n confidential-containers-system
```

## Step 5: Verify IRSA is Working

### 1. Check ServiceAccount Annotations

```bash
# Check cloud-api-adaptor service account
echo "cloud-api-adaptor:"
kubectl get serviceaccount cloud-api-adaptor \
  -n confidential-containers-system \
  -o jsonpath='{.metadata.annotations.eks\.amazonaws\.com/role-arn}'
echo ""

# Check peerpod-ctrl service account (if deployed)
echo "peerpod-ctrl:"
kubectl get serviceaccount peerpodctrl-controller-manager \
  -n confidential-containers-system \
  -o jsonpath='{.metadata.annotations.eks\.amazonaws\.com/role-arn}'
echo ""
```

Expected output:
```
cloud-api-adaptor:
arn:aws:iam::123456789012:role/CAA-IRSA-Role
peerpod-ctrl:
arn:aws:iam::123456789012:role/PeerpodCtrl-IRSA-Role
```

### 2. Check Pod Environment Variables

**For cloud-api-adaptor:**

```bash
CAA_POD=$(kubectl get pods -n confidential-containers-system \
  -l app=cloud-api-adaptor \
  -o jsonpath='{.items[0].metadata.name}')

echo "CAA Pod: ${CAA_POD}"
kubectl exec -n confidential-containers-system ${CAA_POD} -- env | grep AWS
```

Expected output (should include):
```
AWS_REGION=us-east-2
AWS_WEB_IDENTITY_TOKEN_FILE=/var/run/secrets/eks.amazonaws.com/serviceaccount/token
AWS_ROLE_ARN=arn:aws:iam::123456789012:role/CAA-IRSA-Role
AWS_ROLE_SESSION_NAME=...
```

**For peerpod-ctrl (if deployed):**

```bash
CTRL_POD=$(kubectl get pods -n confidential-containers-system \
  -l control-plane=controller-manager \
  -o jsonpath='{.items[0].metadata.name}')

echo "Controller Pod: ${CTRL_POD}"
kubectl exec -n confidential-containers-system ${CTRL_POD} -c manager -- env | grep AWS
```

Expected output (should include):
```
AWS_WEB_IDENTITY_TOKEN_FILE=/var/run/secrets/eks.amazonaws.com/serviceaccount/token
AWS_ROLE_ARN=arn:aws:iam::123456789012:role/PeerpodCtrl-IRSA-Role
AWS_ROLE_SESSION_NAME=...
```

### 3. Test Pod Creation

Create a test pod to verify CAA can create peer pods:

```bash
kubectl run nginx-test \
  --image=nginx \
  --overrides='{"spec":{"runtimeClassName":"kata-remote"}}'

# Watch the pod
kubectl get pod nginx-test -w
```

If the pod starts successfully, IRSA is working correctly!

## Non-EKS Environments

For self-managed Kubernetes clusters or other platforms, you need to:

1. **Set up AWS OIDC provider** - Register your Kubernetes cluster's OIDC issuer with AWS IAM
2. **Configure projected tokens** - Add volume mounts for service account tokens (similar to [Alibaba RRSA implementation](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/src/cloud-api-adaptor/install/charts/peerpods/templates/daemonset.yaml#L69-L128))
3. **Set environment variables manually** in the ConfigMap:
   ```yaml
   providerConfigs:
     aws:
       AWS_WEB_IDENTITY_TOKEN_FILE: "/var/run/secrets/aws/serviceaccount/token"
       AWS_ROLE_ARN: "arn:aws:iam::123456789012:role/CAA-IRSA-Role"
       AWS_REGION: "us-east-1"
       # ... other config
   ```

The AWS SDK will use these environment variables to authenticate via the default credential chain.

## References

- [AWS IRSA Documentation](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
- [Cloud API Adaptor GitHub Issue #3027](https://github.com/confidential-containers/cloud-api-adaptor/issues/3027)
- [EKS Best Practices for Security](https://aws.github.io/aws-eks-best-practices/security/docs/)
- [Kubernetes Service Account Token Volume Projection](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#serviceaccount-token-volume-projection)
