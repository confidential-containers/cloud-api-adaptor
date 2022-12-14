#!/bin/bash
# this script is for develop test only, user must prepared env via wrap-ibm-vpc-block-csi-driver.md 
kubectl delete po nginx
kubectl delete pvc my-pvc
kubectl -n kube-system delete po nginx
kubectl -n kube-system delete pvc my-pvc
pv_name=$(kubectl get pv |grep ibmc-vpc-block-5iops-tier|awk '{print $1}')
kubectl delete pv $pv_name
cd ~/ibm-vpc-block-csi-driver
kustomize build deploy/kubernetes/driver/kubernetes/overlays/stage | kubectl delete -f -
kustomize build deploy/kubernetes/driver/kubernetes/overlays/stage | kubectl apply -f -
cd ~/csi-wrapper
kubectl delete -f crd/peerpodvolume.yaml
kubectl delete -f hack/ibm/vpc-block-csi-wrapper-runner.yaml
kubectl create -f crd/peerpodvolume.yaml
kubectl create -f hack/ibm/vpc-block-csi-wrapper-runner.yaml
make import-csi-controller-wrapper-docker
make import-csi-node-wrapper-docker
make import-csi-podvm-wrapper-docker
kubectl patch statefulset ibm-vpc-block-csi-controller -n kube-system --patch-file hack/ibm/patch-controller.yaml
kubectl patch ds ibm-vpc-block-csi-node -n kube-system --patch-file hack/ibm/patch-node.yaml
