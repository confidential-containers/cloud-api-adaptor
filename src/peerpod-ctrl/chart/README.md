# PeerPod Controller Helm Chart

Helm chart for deploying the PeerPod Controller, which manages the lifecycle of
peer pod cloud resources and cleans up dangling VMs when cloud-api-adaptor fails
to delete them.

> **Note**: This chart is automatically installed as a dependency of the main
> `peerpods` chart. In most cases, you don't need to install it manually.
> See the [peerpods chart documentation](../peerpods/README.md) for standard installation.

## Prerequisites

Before installing this chart, ensure you have:

- **Helm** v3.x or v4.x installed ([installation guide](https://helm.sh/docs/intro/install/))
- **Kubernetes cluster** with appropriate access
- **kubeconfig** configured to access your cluster

## Quick Start

### Standalone Installation

```bash
# Install the chart
helm install peerpodctrl ./chart \
  -n confidential-containers-system \
  --create-namespace
```

### Installation as Dependency

This chart is typically installed as a dependency of the main `peerpods` chart.
See the [peerpods chart documentation](../cloud-api-adaptor/install/charts/peerpods/README.md)
for installation instructions.

## Configuration

All configuration options are documented with inline comments in `values.yaml`.

To customize the installation, you can either:
1. Edit `values.yaml` directly
2. Create your own values file and pass it with `-f`
3. Override specific values with `--set`

Example with custom namespace:

```bash
helm install my-controller ./chart \
  --set namespace=my-namespace \
  --set namePrefix=my-controller- \
  -n my-namespace \
  --create-namespace
```

## Auto-Generated Manifests

This chart includes auto-generated resources:

- **CRD** (`crds/confidentialcontainers.org_peerpods.yaml`) - Generated from Go types
- **RBAC** (`templates/rbac.yaml`) - Generated from RBAC markers in Go code

These files are automatically regenerated when running:

```bash
make manifests
```

> **Note**: The generated files are committed to git so users can install the
> chart without running `make manifests`. Developers should run this command
> after modifying the PeerPod CRD Go types or RBAC markers.

## Uninstall

```bash
helm uninstall peerpodctrl -n confidential-containers-system
```
