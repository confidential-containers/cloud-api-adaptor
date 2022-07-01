#!/bin/bash

kubectl apply -f ext-res-rbac.yaml
kubectl apply -f ext-res-cm.yaml
kubectl apply -f ext-res-ds.yaml
