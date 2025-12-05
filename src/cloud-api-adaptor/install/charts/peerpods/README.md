# PeerPods Helm Chart

> [!WARNING]
> **Experimental**: This chart is subject to change. The final location for
> this chart has not been decided. It may move to the CAA repository, the
> operator repository, or its own repository.

Experimental Helm chart for deploying Cloud API Adaptor (CAA) controllers and
PeerPods configuration.

## Quick Start

### 1. Setup secrets

```bash
cp providers/<provider>-secrets.yaml.template <provider>-secrets.yaml
# Edit <provider>-secrets.yaml with your credentials
# WARNING: Do not commit this file to git!
```

### 2. Install with provider config

```bash
helm install peerpods . \
  -f providers/<provider>.yaml \
  -f <provider>-secrets.yaml \
  -n confidential-containers-system \
  --create-namespace
```

## Configuration

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
