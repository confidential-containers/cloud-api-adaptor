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

## Quick Start

### Option A: Development/Testing (secrets.mode: create)

In this mode, Helm creates the K8s Secret from values you provide.

> **Warning**: Secrets passed via `helm install -f` are stored in Helm release
> history and can be retrieved with `helm get values`. Not recommended for production.

```bash
# 1. Copy and fill in the secrets template
cp providers/<provider>-secrets.yaml.template <provider>-secrets.yaml
# Edit <provider>-secrets.yaml with your credentials
# WARNING: Do not commit this file to git!

# 2. Install with provider config and secrets
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

```bash
# 1. Create the secret externally (kubectl, Vault, External Secrets Operator, etc.)
# See providers/<provider>-secrets.yaml.template for required keys
kubectl create secret generic my-provider-creds \
  -n confidential-containers-system \
  --from-literal=AZURE_CLIENT_ID='...' \
  --from-literal=AZURE_CLIENT_SECRET='...' \
  --from-literal=AZURE_TENANT_ID='...'

# 2. Install referencing the existing secret
helm install peerpods . \
  -f providers/<provider>.yaml \
  --set secrets.mode=reference \
  --set secrets.existingSecretName=my-provider-creds \
  --dependency-update \
  -n confidential-containers-system \
  --create-namespace
```

## Configuration

### Image Tags

Two image variants are published:

| Variant | Tag Format | Includes CGO | Used By |
|---------|------------|--------------|---------|
| Dev | `latest` or `dev-<commit>` | Yes | libvirt, docker |
| Release | `<commit>` | No | aws, azure, gcp, ibmcloud, etc. |

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

```bash
# Generate for all providers
cd src/cloud-providers && make sync-chart-values

# Generate for a specific provider only
cd src/cloud-providers && make sync-chart-values CLOUD_PROVIDER=<provider>
```

Each provider file contains all available configuration options with their
defaults and descriptions. Required fields are uncommented; optional fields
are commented out with their default values.

You can override any value by either:
1. Editing the provider file directly (CI will check for drift)
2. Creating your own values file and passing it with `-f`

## Uninstall

```bash
helm uninstall peerpods -n confidential-containers-system
```
