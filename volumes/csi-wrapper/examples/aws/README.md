# AWS EBS CSI Wrapper for Peer Pod Storage

## Prerequisites

* Running Kubernetes cluster (Version >= 1.20) on AWS

* Peer-Pods is [deployed](../../../../aws/README.md)

## AWS EBS CSI Driver Installation

**NOTE:** the following is just a basic example, follow official [installation instructions](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/install.md) for advanced configuration.

1. Create IAM Policy
```
aws iam create-policy \
	--policy-name EBS_Policy \
	--policy-document file://example-iam-policy.json
```
2.  Grant the driver IAM permissions:
```
kubectl create secret generic aws-secret \
    --namespace kube-system \
    --from-literal "key_id=${AWS_ACCESS_KEY_ID}" \
    --from-literal "access_key=${AWS_SECRET_ACCESS_KEY}"
```
3. Deploy the driver:
```
kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.21"
```
4. Verify the pods are running:
```
kubectl get pods -n kube-system -l app.kubernetes.io/name=aws-ebs-csi-driver
```

## Apply the PeerPods CSI wrapper

1. Create the PeerpodVolume CRD object
```
kubectl apply -f ../../crd/peerpodvolume.yaml
```
2. Apply RBAC roles to permit the wrapper to execute the required operations
```
kubectl apply -f rbac-ebs-csi-wrapper-runner.yaml
kubectl apply -f rbac-ebs-csi-wrapper-podvm.yaml
```
3. Patch the EBS CSI Driver:
```
kubectl patch deploy ebs-csi-controller -n kube-system --patch-file patch-controller.yaml
kubectl -n kube-system delete replicaset -l app=ebs-csi-controller
kubectl patch ds ebs-csi-node -n kube-system --patch-file patch-node.yaml
```
4. Verify the pods are running (each pod should contain an additional container):
```
kubectl get pods -n kube-system -l app.kubernetes.io/name=aws-ebs-csi-driver
```

## Example Workload With Provisioned Volume

This is based on the Dynamic Volume Provisioning [example](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/tree/master/examples/kubernetes/dynamic-provisioning)

1. Deploy example pod on your cluster along with the StorageClass and PersistentVolumeClaim:
```
 kubectl apply -f dynamic-provisioning/
```
2. Validate the PersistentVolumeClaim is bound to your PersistentVolume:
```
kubectl get pvc ebs-claim
```
3. Once the pod is running you can validate some date (timestamps) has been written to the dynamically provisioned volume:
```
kubectl exec app -- cat /data/out.txt
```
4. Cleanup resources:
```
kubectl delete -f dynamic-provisioning/
```
