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
cp values-secrets.yaml.template values-secrets.yaml
# Edit values-secrets.yaml with your credentials (gitignored)
```

### 2. Configure provider

Edit `values.yaml`:
```yaml
provider: libvirt  # or aws, azure, gcp, etc.
```

### 3. Install

```bash
helm install peerpods . \
  -f values.yaml \
  -f values-secrets.yaml \
  -n confidential-containers-system \
  --create-namespace
```

## Configuration

- **Non-sensitive config** → `values.yaml` (committed)
- **Secrets/credentials** → `values-secrets.yaml` (gitignored)

See `values.yaml` for provider-specific configuration options.

## Uninstall

```bash
helm uninstall peerpods -n confidential-containers-system
```
