#!/bin/bash
# this script is for develop test only, user must prepare value of slclient.toml via README.md
# make the `REPLACE_ME` is replaced in ibm-vpc-block-csi-driver-master.yaml
kubectl delete po nginx
kubectl delete pvc my-pvc
kubectl -n kube-system delete po nginx
kubectl -n kube-system delete pvc my-pvc
pv_name=$(kubectl get pv |grep ibmc-vpc-block-5iops-tier|awk '{print $1}')
kubectl delete pv $pv_name

kubectl delete -f ./ibm-vpc-block-csi-driver-v5.2.0.yaml
kubectl delete -f ../../crd/peerpodvolume.yaml
kubectl delete -f ./vpc-block-csi-wrapper-runner.yaml
kubectl create -f ./ibm-vpc-block-csi-driver-v5.2.0.yaml
kubectl create -f ../../crd/peerpodvolume.yaml
kubectl create -f ./vpc-block-csi-wrapper-runner.yaml

kubectl patch Deployment ibm-vpc-block-csi-controller -n kube-system --patch-file ./patch-controller.yaml
kubectl patch ds ibm-vpc-block-csi-node -n kube-system --patch-file ./patch-node.yaml
