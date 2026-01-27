# PeerPods Helm Chart

> [!WARNING]
> **Experimental**: This chart is subject to change. The final location for
> this chart has not been decided. It may move to the CAA repository, the
> operator repository, or its own repository.

Experimental Helm chart for deploying Cloud API Adaptor (CAA) controllers and
PeerPods configuration.

## Prerequisites

Before installing this chart, ensure you have:

- **Helm** v3.x or v4.x installed ([installation guide](https://helm.sh/docs/intro/install/))
- **Kubernetes cluster** with appropriate access
- **kubeconfig** configured to access your cluster
- **cert-manager** installed (required for webhook, enabled by default)

> [!NOTE]
> The webhook handles proper resource scheduling for peer-pods. Disabling it
> (`--set webhook.enabled=false`) is only recommended for development or when
> worker nodes have sufficient resources.

## Installation from OCI Registry

The chart is published to GitHub Container Registry with cryptographic attestations for supply chain security.

### Verifying Chart Authenticity

Before installing, you can verify the chart was built by the official GitHub Actions workflow:

```bash
gh attestation verify oci://ghcr.io/confidential-containers/cloud-api-adaptor/charts/peerpods:0.1.0-dev \
  --owner confidential-containers
```

### Installing from Registry

```bash
helm install peerpods oci://ghcr.io/confidential-containers/cloud-api-adaptor/charts/peerpods \
  --version 0.1.0-dev \
  -f providers/<provider>.yaml \
  -n confidential-containers-system \
  --create-namespace
```

## Quick Start (Development)

### Option A: Development/Testing (secrets.mode: create)

In this mode, Helm creates the K8s Secret from values you provide.

> **Warning**: Secrets passed via `helm install -f` are stored in Helm release
> history and can be retrieved with `helm get values`. Not recommended for production.

1. Copy the secrets template
    ```bash
    cp providers/<provider>-secrets.yaml.template <provider>-secrets.yaml
    ```
2. Edit `<provider>-secrets.yaml` with your credentials
    > **Warning**: Do not commit this file to git!

3. Install with provider config and secrets
    ```bash
    helm install peerpods . \
      -f providers/<provider>.yaml \
      -f <provider>-secrets.yaml \
      --dependency-update \
      -n confidential-containers-system \
      --create-namespace
    ```

### Option B: Production (secrets.mode: reference)

In this mode, you create the K8s Secret externally and Helm only references it by name.
Secrets never flow through Helm.

1. Create namespace:
    ```bash
   kubectl apply -f - << EOF
   apiVersion: v1
   kind: Namespace
   metadata:
     name: confidential-containers-system
     labels:
       app.kubernetes.io/managed-by: Helm
     annotations:
       meta.helm.sh/release-name: peerpods
       meta.helm.sh/release-namespace: confidential-containers-system
   EOF
    ```

2. Create the secret externally (kubectl, Vault, External Secrets Operator, etc.)<br>
   See `providers/<provider>-secrets.yaml.template` for required keys.
   - Microsoft Azure example:
     ```bash
     kubectl create secret generic my-provider-creds \
       -n confidential-containers-system \
       --from-literal=AZURE_CLIENT_ID='...' \
       --from-literal=AZURE_CLIENT_SECRET='...' \
       --from-literal=AZURE_TENANT_ID='...'
     ```

   - Google Cloud (GCP) example:
     ```bash
     kubectl create secret generic my-provider-creds \
        -n confidential-containers-system \
        --from-file=GCP_CREDENTIALS=<PATH_TO_YOUR_CREDENTIALS_JSON_FILE>
     ```

3. Install referencing the existing secret
    ```bash
    helm install peerpods . \
      -f providers/<provider>.yaml \
      --set secrets.mode=reference \
      --set secrets.existingSecretName=my-provider-creds \
      --dependency-update \
      -n confidential-containers-system \
      --create-namespace
    ```

   > **Note**: To verify the installation without applying to the cluster, you can render
   the templates by replacing `helm install` with `helm template`.
    
## Configuration

### Image Tags

Two image variants are published:

| Variant | Tag Format                 | Includes CGO | Used By                         |
|---------|----------------------------|--------------|---------------------------------|
| Dev     | `latest` or `dev-<commit>` | Yes          | libvirt, docker                 |
| Release | `<commit>`                 | No           | aws, azure, gcp, ibmcloud, etc. |

- **During development**: `latest` works for all providers (default in values.yaml)
- **At release**: `values.yaml` is updated to `<commit>`, and libvirt/docker provider files override with `dev-<commit>`

The libvirt and docker provider files automatically include an `image.tag` override
since they require the dev image with CGO bindings.

### File Structure

- `values.yaml` - Base defaults (namespace, limits, etc.)
- `providers/<provider>.yaml` - Provider-specific configuration (auto-generated)
- `providers/<provider>-secrets.yaml.template` - Secrets template (auto-generated)

### Provider Values

Provider-specific values files are located in the `providers/` directory.
These files are auto-generated from the cloud provider source code using:

- Generate for all providers 
   ```bash
   pushd ../../../../cloud-providers && \
   make sync-chart-values && \
   popd
   ```

- Generate for a specific provider only
   ```bash
   pushd ../../../../cloud-providers && \
   make sync-chart-values CLOUD_PROVIDER=<provider> && \
   popd
   ```

Each provider file contains all available configuration options with their defaults and descriptions. 

- **Required** fields are uncommented,
- **Optional** fields are commented out with their default values.

You can override any value by either:

1. Editing the provider file directly (CI will check for drift)
2. Creating your own values file and passing it with `-f`

## Uninstall

To uninstall and remove all resources created by this chart, run:

1. Uninstall the Helm release
    ```bash
    helm uninstall peerpods -n confidential-containers-system
    ```

2. Remove secrets if it was created externally (`secrets.mode`: `reference`)
    ```bash
    kubectl delete secret my-provider-creds \
      -n confidential-containers-system
    ```

3. Delete the namespace (optional)
    ```bash
    kubectl delete ns confidential-containers-system
    ```