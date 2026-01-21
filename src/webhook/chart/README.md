# PeerPods Webhook Helm Chart

Helm chart for deploying the PeerPods mutating admission webhook, which intercepts
pod creation requests and modifies pod specs to use peer pods runtime and resources.

## Prerequisites

Before installing this chart, ensure you have:

- **Helm** v3.x or v4.x installed ([installation guide](https://helm.sh/docs/intro/install/))
- **Kubernetes cluster** with appropriate access
- **kubeconfig** configured to access your cluster

> [!WARNING]
> The webhook requires TLS certificates to operate. This chart uses cert-manager
> to automatically generate and manage these certificates.
>
> By default, cert-manager will be installed automatically (`certManager.install=true`).
> If cert-manager is already installed in your cluster, set `--set certManager.install=false`.
>
> To disable cert-manager integration entirely and provide certificates manually,
> set `--set certManager.enabled=false`.

## Quick Start

### Standalone Installation

**With automatic cert-manager installation (default):**

```bash
helm install peerpods-webhook ./chart \
  -n confidential-containers-system \
  --create-namespace
```

**If cert-manager is already installed:**

```bash
helm install peerpods-webhook ./chart \
  --set certManager.install=false \
  -n confidential-containers-system \
  --create-namespace
```

### Installation as Dependency

This chart is typically installed as a dependency of the main `peerpods` chart.
See the [peerpods chart documentation](../../cloud-api-adaptor/install/charts/peerpods/README.md)
for installation instructions.

## Configuration

All configuration options are documented with inline comments in `values.yaml`.

To customize the installation, you can either:
1. Edit `values.yaml` directly
2. Create your own values file and pass it with `-f`
3. Override specific values with `--set`

Example with custom runtime class:

```bash
helm install my-webhook ./chart \
  --set webhook.targetRuntimeClass=kata-qemu \
  --set webhook.podVMExtendedResource=kata.peerpods.io/vm \
  -n confidential-containers-system \
  --create-namespace
```

## Auto-Generated Manifests

This chart includes auto-generated resources:

- **MutatingWebhookConfiguration** (`templates/webhook-config.yaml`) - Generated from webhook markers in Go code

This file is automatically regenerated when running:

```bash
make manifests
```

> **Note**: The generated file is committed to git so users can install the
> chart without running `make manifests`. Developers should run this command
> after modifying the webhook markers in the Go code.

## Uninstall

```bash
helm uninstall peerpods-webhook -n confidential-containers-system
```
