# Cloud API Adaptor on GCP

This documentation will walk you through setting up Cloud API Adaptor (CAA) on
Google Compute Engine (GCE). We will build the pod vm image, CCA dev image and
experiment on a local libvirt cluster.

## Build Pod VM Image

### Modifying existing marketplace image

Install packer by following [these
instructions](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).

Create 'packer' service account with compute instance admin role:

```bash
export GCP_PROJECT_ID="REPLACE_ME"
gcloud iam service-accounts create packer \
 --project ${GCP_PROJECT_ID} \
  --description="Packer Service Account" \
  --display-name="Packer Service Account"

gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} \
  --member=serviceAccount:packer@${GCP_PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/compute.instanceAdmin.v1

gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} \
  --member=serviceAccount:packer@${GCP_PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/iam.serviceAccountUser
```

Create application access token:

```bash
gcloud iam service-accounts keys create ${HOME}/.config/gcloud/packer_application_key.json \
  --iam-account=packer@${GCP_PROJECT_ID}.iam.gserviceaccount.com

export GOOGLE_APPLICATION_CREDENTIALS=${HOME}/.config/gcloud/packer_application_key.json
```

Create a custom GCP VM image based on Ubuntu 20.04 having kata-agent,
agent-protocol-forwarder and other dependencies.

```bash
cd image
export GCP_ZONE="REPLACE_ME" # e.g. "us-west1-a"
export GCP_MACHINE_TYPE="REPLACE_ME" # default is "e2-medium"
export GCP_NETWORK="REPLACE_ME" # default is "default"
export CLOUD_PROVIDER=gcp
PODVM_DISTRO=ubuntu make image && cd -
```

You can also build the image using docker:

```bash
cd image
DOCKER_BUILDKIT=1 docker build -t gcp \
  --secret id=APPLICATION_KEY,src=${GOOGLE_APPLICATION_CREDENTIALS} \
  --build-arg GCP_PROJECT_ID=${GCP_PROJECT_ID} \
  --build-arg GCP_ZONE=${GCP_ZONE} \
  -f Dockerfile .
```

## Local experimentation

### Setup a local cluster

For local development we'll use libvirt setup and a local Docker registry.
Run local Docker registry:

```bash
docker stop registry
docker rm registry
docker run -d -p 5000:5000 --restart=always --name registry registry:2.7.0
```

Follow instructions in [libvirt/README.md](../libvirt/README.md) and deploy a
Kubernetes cluster in a Kubeadm setup.

```
$ kcli list kube
+-----------+---------+-----------+-----------------------------------------+
|  Cluster  |   Type  |    Plan   |                  Vms                    |
+-----------+---------+-----------+-----------------------------------------+
| peer-pods | generic | peer-pods | peer-pods-ctlplane-0,peer-pods-worker-0 |
+-----------+---------+-----------+-----------------------------------------+

$ kubectl get nodes
NAME                 STATUS   ROLES                  AGE     VERSION
peer-pods-ctlplane-0 Ready    control-plane,master   6m8s    v1.25.3
peer-pods-worker-0   Ready    worker                 2m47s   v1.25.3
```

### Deploy the CoCo operator

Deploy operator (copied from `deploy` target in [Makefile](../Makefile)):

```bash
kubectl apply -k "github.com/confidential-containers/operator/config/default"
kubectl apply -k "github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods"
```

Register "insecure" local Registry in worker node:

```bash
$ kcli ssh
...
ubuntu@peer-pods-worker-0:~$ sudo vim /etc/containerd/config.toml
```

Add the following:

```
[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."192.168.122.1:5000"]
      endpoint = ["http://192.168.122.1:5000"]
  [plugins."io.containerd.grpc.v1.cri".registry.configs]
    [plugins."io.containerd.grpc.v1.cri".registry.configs."192.168.122.1:5000".tls]
      insecure_skip_verify = true
```

Restart `containerd`:

```bash
ubuntu@peer-pods-worker-0:~$ sudo systemctl restart containerd
```

Now the worker node can download local dev CCA images from the local registry running on the host.

### Build CCA dev image

Build and push docker image to local registry:

```bash
export registry=192.168.122.1:5000
export RELEASE_BUILD=true
make image
```

### Configure VPC network

Go to your GCP console, under "VPC networks" update the `default` (global)
network and add a firewall rule that allows incoming TCP connections over port
15150 from your workstation external IP address.

## Deploy CCA

Update [install/overlays/gcp/kustomization.yaml](../install/overlays/gcp/kustomization.yaml) with the required fields:

```
images:
- name: cloud-api-adaptor
  newName: 192.168.122.1:5000/cloud-api-adaptor # change image if needed
  newTag: 47dcc2822b6c2489a02db83c520cf9fdcc833b3f-dirty # change if needed
  ...
configMapGenerator:
- name: peer-pods-cm
  namespace: confidential-containers-system
  literals:
  - CLOUD_PROVIDER="gcp" # leave as is.
  - PODVM_IMAGE_NAME="" # set from step 1 above.
  - GCP_PROJECT_ID="" # set
  - GCP_ZONE="" # set
  - GCP_MACHINE_TYPE="e2-medium" # defaults to e2-medium
  - GCP_NETWORK="global/networks/default" # leave as is.
  ...
secretGenerator:
  ...
- name: peer-pods-secret
  namespace: confidential-containers-system
  files:
  - GCP_CREDENTIALS # Make sure this file has the application creadentials. You can reuse the Packer creds: copy the file from ${HOME}/.config/gcloud/packer_application_key.json
```

```bash
$ kubectl apply -k install/overlays/gcp/
$ kubectl get pods -n confidential-containers-system
NAME                                              READY   STATUS    RESTARTS   AGE
cc-operator-controller-manager-546574cf87-b69pb   2/2     Running   0          7d10h
cc-operator-daemon-install-mfjbj                  1/1     Running   0          7d10h
cc-operator-pre-install-daemon-g2jsj              1/1     Running   0          7d10h
cloud-api-adaptor-daemonset-5w8nw                 1/1     Running   0          7s
```

## Test with a simple workflow

Deploy the `sample_busybox.yaml` (see [libvirt/README.md](../libvirt/README.md)):

```
$ kubectl apply -f sample_busybox.yaml
pod/busybox created
```

Examine your GCP console, under "Compute Engine", "VM instances" you should see the new POD instance running.
Examine `kubectl logs` and verify the tunnel to the podvm was established successfully.
