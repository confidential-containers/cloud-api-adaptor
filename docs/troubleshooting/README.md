# Troubleshooting

This document describes generic troubleshooting steps after installation of Cloud API Adaptor.

## Pod created but it stays in `ContainerCreating` state

```console
$ kubectl get pods -n confidential-containers-system -o wide
NAME                                              READY   STATUS    RESTARTS   AGE     IP            NODE                                NOMINATED NODE   READINESS GATES
cc-operator-controller-manager-585ffdbffd-2bmg6   2/2     Running   0          6m25s   10.244.0.11   aks-nodepool1-17035871-vmss000000   <none>           <none>
cc-operator-daemon-install-bskth                  1/1     Running   0          6m21s   10.244.0.12   aks-nodepool1-17035871-vmss000000   <none>           <none>
cloud-api-adaptor-daemonset-xxvtt                 1/1     Running   0          6m22s   10.224.0.4    aks-nodepool1-17035871-vmss000000   <none>           <none>
```

It is possible that the `cloud-api-adaptor-daemonset` is not deployed correctly. To see what is wrong with it run the following command and look at the events to get insights:

```console
$ kubectl -n confidential-containers-system describe ds cloud-api-adaptor-daemonset
Name:           cloud-api-adaptor-daemonset
Selector:       app=cloud-api-adaptor
Node-Selector:  node-role.kubernetes.io/worker=
...
Events:
  Type    Reason            Age    From                  Message
  ----    ------            ----   ----                  -------
  Normal  SuccessfulCreate  8m13s  daemonset-controller  Created pod: cloud-api-adaptor-daemonset-xxvtt
```

But if the `cloud-api-adaptor-daemonset` is up and in `Running` state, like shown above then look at the pods' logs, for more insights:

```bash
kubectl -n confidential-containers-system logs cloud-api-adaptor-daemonset-xxvtt
```

Note that this is a single node cluster. So there is only one `cloud-api-adaptor-daemonset-*` pod. But if you are running on a multi-node cluster then look for the node your workload fails to come up and only see the logs of corresponding CAA pod.
