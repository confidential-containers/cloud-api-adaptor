# Cloud API Adaptor on GCP

This documentation will walk you through setting up Cloud API Adaptor (CAA)
on Google Compute Engine (GCE). We will build the pod vm image.

## Build Pod VM Image

### Option-1: Modifying existing marketplace image
Install packer by following [these instructions](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).

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

Create a custom GCP VM image based on Ubuntu 20.04 having kata-agent, agent-protocol-forwarder and other dependencies.

```
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
