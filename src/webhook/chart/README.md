# PeerPods Webhook Helm Chart

Helm chart for deploying the PeerPods mutating admission webhook, which intercepts
pod creation requests and modifies pod specs to use peer pods runtime and resources.

## Prerequisites

Before installing this chart, ensure you have:

- **Helm** v3.x or v4.x installed ([installation guide](https://helm.sh/docs/intro/install/))
- **Kubernetes cluster** with appropriate access
- **kubeconfig** configured to access your cluster
- **cert-manager** installed in the cluster ([installation guide](https://cert-manager.io/docs/installation/))

> **Note**: The webhook requires TLS certificates to operate. This chart uses cert-manager
> to automatically generate and manage these certificates.

## Quick Start

### Standalone Installation

```bash
# Install cert-manager if not already installed
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.3/cert-manager.yaml

# Install the webhook chart
helm install peerpods-webhook ./chart \
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

## Admission controllers that rewrite the webhook configuration

Some managed clusters run an admission controller that edits
`MutatingWebhookConfiguration` objects. On AKS the admissions enforcer adds a
`namespaceSelector` that keeps the webhook away from `kube-system` and the AKS
internal namespaces, which is the behaviour you want. With server-side apply
(the default for releases installed with Helm 4) the next `helm upgrade` then
fails with a field ownership conflict against the `admissionsenforcer` manager.
Let Helm's apply proceed:

```bash
helm upgrade my-webhook ./chart --force-conflicts ...
```

`webhook.annotations` and `webhook.labels` are rendered on the
`MutatingWebhookConfiguration` for clusters that need it marked. AKS honours
`admissions.enforcer/disabled: "true"` as an opt-out, but that lets the webhook
act on AKS internal namespaces, and with `failurePolicy: Fail` a webhook outage
then blocks pod creation there.

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
