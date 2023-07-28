# Secure Key Relase (SKR) at Runtime 

## Architecture

SKR is deployed as 2 components:

### attester

A privileged process, interacting with TEE platform services and devices. It exposes a simple API to gather TEE evidence via   Unix Domain Socket. It is packaged with the podvm image and runs as a systemd unit. `agent-protocol-forwarder` will inject the process' socket into each container as a mount entry.

### skr-api

A process that can be deployed as a sidecar in a pod, it'll access the `attester` socket to retrieve TEE evidence from the podvm in a remote attestation flow with an external KBS/verifier. It exposes a simple http API on `localhost:50080` for other pods to use.

## Build

The attester component is built as part of the podvm image build. skr-api can be via Docker:

```bash
export my_kbs="https://mykbs.com"
export my_image="example.com/image:tag"
docker build -t "$my_image" .
docker push "$my_image" 
cat << EOF > nginx-caa.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx-caa
  name: nginx-caa
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx-caa
  template:
    metadata:
      labels:
        app: nginx-caa
    spec:
      runtimeClassName: kata-remote
      containers:
      - image: nginx:stable
        name: nginx
      - image: $my_image
        name: skr-api
        command: ["/skr-api", "-k", "$my_kbs" ]
EOF
kubectl apply -f nginx-caa.yaml
```

## Test

Assuming there is a functional kbs deployment with a resource `one/two/key` in its repository:

```bash
$ kubectl get po
NAME                         READY   STATUS    RESTARTS   AGE
nginx-caa-7f65ff5595-9j2vm   2/2     Running   0          78m
$ kubectl exec -it -c nginx nginx-caa-7f65ff5595-9j2vm -- curl localhost:50080/getresource/one/two/key
my_secret
```
