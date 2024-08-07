# Cloud API Adaptor on GCP

This documentation will walk you through setting up Cloud API Adaptor (CAA) on
Google Compute Engine (GCE). We will build the pod vm image, CCA dev image and
experiment on a local libvirt cluster (or a cluster running on a GCP VM).

## Build Pod VM Image

### Prep-work: Google project

Create a project with your google console dashboard, install gcloud and run the
following commands:

### Create a Google service account and credentials

```bash
export GCP_PROJECT_ID="REPLACE_ME"
gcloud auth login
gcloud config set project ${GCP_PROJECT_ID}
```

Create a 'peerpods' service account with compute instance admin role:

```bash
gcloud iam service-accounts create peerpods \
 --project ${GCP_PROJECT_ID} \
  --description="Peerpods Service Account" \
  --display-name="Peerpods Service Account"

gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} \
  --member=serviceAccount:peerpods@${GCP_PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/compute.instanceAdmin.v1

gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} \
  --member=serviceAccount:peerpods@${GCP_PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/iam.serviceAccountUser
```

Create application access token:

```bash
gcloud iam service-accounts keys create ${HOME}/.config/gcloud/peerpods_application_key.json \
  --iam-account=peerpods@${GCP_PROJECT_ID}.iam.gserviceaccount.com

export GOOGLE_APPLICATION_CREDENTIALS=${HOME}/.config/gcloud/peerpods_application_key.json
```

### Build the Podvm image and upload

Create a custom GCP VM image having kata-agent, agent-protocol-forwarder and
other dependencies, using mkosi:

```bash
cd podvm-mkosi/
make binaries
make image
cp build/system.raw build/disk.raw
tar -cvzf build/disk.tar.gz -C build disk.raw
```

> [!NOTE]
> For details about mkosi, visit the `podvm-mkosi/README.md` file.

Create the bucket to store the disk:


```bash
export GCP_LOCATION="us-west1" # or anyother here
export BUCKET_NAME="peerpods-bucket"
gsutil mb -p ${GCP_PROJECT_ID} -l ${GCP_LOCATION} gs://${BUCKET_NAME}/
```

Upload the disk image to a bucket and create the image:

```bash
gsutil cp build/disk.tar.gz gs://${BUCKET_NAME}/peerpods-disk.tar.gz
gcloud compute images create podvm-image \
   --source-uri=gs://${BUCKET_NAME}/peerpods-disk.tar.gz \
   --guest-os-features=UEFI_COMPATIBLE
```

## Deployment

### Setup a cluster

You will need a k8s cluster up and running. Since GKE support is blocked by the
issue #1909, you will need to bootstrap a cluster, either local (with libvirt)
or using Google Compute Engine.

For local development, you could use a local libvirt setup with a local Docker
registry (optional). Follow instructions in
[libvirt/README.md](../libvirt/README.md) and deploy a Kubernetes cluster in a
Kubeadm setup.

Regardless of the method used, at the end you should have a KUBECONFIG pointing
to the auth file and a cluster up and running:

```
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

> [!NOTE]
> If you are using a local registry instead of quay.io, you need to register
> "insecure" local Registry in your worker node.
> TODO: Better document this process in a different file.

### Build CCA dev image

Build and push docker image to a registry:

```bash
export registry=quay.io/${QUAY_USER} # If you are using local registry: LOCAL_IP:PORT.
export RELEASE_BUILD=true
make image
```

After that you should take note of the tag used for this image, we will use it
later.

### Configure VPC network

We need to make sure port 15150 is open under the `default` VPC network:

```
gcloud compute firewall-rules create allow-port-15150 \
    --project=${GCP_PROJECT_ID} \
    --network=default \
    --allow=tcp:15150
```

> [!NOTE]
> For production scenarios, it is advisable to restrict the source IP range to
> minimize security risks. For example, you can restrict the source range to a
> specific IP address or CIDR block:
>
> ```
>  gcloud compute firewall-rules create allow-port-15150-restricted \
>    --project=${PROJECT_ID} \
>    --network=default \
>    --allow=tcp:15150 \
>    --source-ranges=[YOUR_EXTERNAL_IP]
> ```

## Deploy CCA

Update [install/overlays/gcp/kustomization.yaml](../install/overlays/gcp/kustomization.yaml) with the required fields:

```
images:
- name: cloud-api-adaptor
  newName: 192.168.122.1:5000/cloud-api-adaptor # change image if needed
  newTag: 47dcc2822b6c2489a02db83c520cf9fdcc833b3f-dirty # change to your tag
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
  - GCP_CREDENTIALS # Make sure this file has the application creadentials. You can use the Peerpods creds: copy the file from ${HOME}/.config/gcloud/peerpods_application_key.json
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
