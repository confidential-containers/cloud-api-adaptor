# TLS profile configuration for peer pod connections

`cloud-api-adaptor` (CAA) connects to `agent-protocol-forwarder` (APF) running inside each peer pod VM using mutual TLS. By default, Go's TLS stack enforces a minimum of TLS 1.2 and selects cipher suites automatically. Two environment variables let operators override these defaults to meet organisational security policy or regulatory requirements.

| Variable | Flag equivalent | Default |
|---|---|---|
| `TLS_MIN_VERSION` | `--tls-min-version` | `""` (Go default: TLS 1.2) |
| `TLS_CIPHER_SUITES` | `--tls-cipher-suites` | `""` (Go default cipher selection) |

## Why these variables exist

Kubernetes platform administrators often need to enforce a cluster-wide TLS policy — for example, prohibiting TLS 1.2 entirely, or restricting cipher suites to exclude CBC-mode ciphers and require forward secrecy. On OpenShift, this is controlled by the cluster `TLSSecurityProfile` and injected into operators via environment variables.

These variables allow CAA to honour the same TLS policy that governs the rest of the cluster, ensuring that peer pod connections are not a weaker link in the overall TLS posture.

## Who should configure these

These variables are normally injected automatically by the platform operator (e.g. the OpenShift Sandboxed Containers operator reads the cluster TLS profile and sets them on the CAA DaemonSet). Manual configuration is appropriate for:

- Bare Kubernetes deployments where no operator manages the TLS profile.
- Testing and validation of a specific TLS configuration.
- Compliance audits that require explicit enforcement beyond Go defaults.

End users running workloads in peer pods do not need to configure these variables.

## How the profile is applied

The TLS profile is **baked into the peer pod VM at creation time** via user-data passed to the VM on boot. Once a VM boots, the profile written to its `apf.json` is immutable. This means:

- Profile changes only take effect for **newly created** peer pods.
- Existing running peer pods are not affected; they continue using the profile they were created with.
- To apply a changed profile to existing pods, delete them so they are recreated.

CAA validates the profile at startup. If an invalid combination is provided (for example, cipher suites specified alongside `VersionTLS13`), CAA will refuse to start with a clear error message rather than failing silently on the first pod creation attempt.

## Constraints

- `TLS_MIN_VERSION` accepts `VersionTLS12` or `VersionTLS13`. TLS 1.0 and 1.1 are rejected.
- `TLS_CIPHER_SUITES` must not be set when `TLS_MIN_VERSION` is `VersionTLS13`. Go's `crypto/tls` does not allow configuring cipher suites for TLS 1.3 — its cipher suite set is fixed by the specification.
- Cipher suite names must use IANA names (e.g. `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`), not OpenSSL names.
- When `TLS_CIPHER_SUITES` is set, it applies only to TLS 1.2 connections; TLS 1.3 connections always use Go's built-in cipher suite set.

## Examples

### Require TLS 1.3 minimum

```yaml
# install/charts/peerpods/providers/libvirt.yaml (or any provider overlay)
tlsProfile:
  minVersion: "VersionTLS13"
```

This prevents TLS 1.2 from being negotiated on any new peer pod connection. No cipher suite configuration is needed or allowed.

### Require TLS 1.2 with GCM-only cipher suites (no CBC)

```yaml
tlsProfile:
  minVersion: "VersionTLS12"
  cipherSuites: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"
```

This corresponds to the OpenShift `Intermediate` TLS profile. CBC cipher suites are excluded.

### Set via `kubectl` directly on the peer-pods-cm ConfigMap

```bash
kubectl patch cm peer-pods-cm -n confidential-containers-system \
  --type merge \
  -p '{"data":{"TLS_MIN_VERSION":"VersionTLS13"}}'
kubectl rollout restart daemonset cloud-api-adaptor-daemonset \
  -n confidential-containers-system
```

Note: after restarting CAA, delete and recreate any peer pods for the new profile to take effect.

### Set via environment variables on the CAA DaemonSet

```bash
kubectl set env daemonset/cloud-api-adaptor-daemonset \
  -n confidential-containers-system \
  TLS_MIN_VERSION=VersionTLS12 \
  TLS_CIPHER_SUITES=TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
```

## Verifying the configuration

After creating a new peer pod, confirm the profile was baked in by inspecting the APF config on the running VM:

```bash
# Find the VM IP (example using virsh for libvirt)
virsh -c qemu:///system domifaddr <podvm-name>

# SSH into the VM and check apf.json
ssh root@<vm-ip> 'grep -E "tls-min|tls-cipher" /run/peerpod/apf.json'
```

Expected output for TLS 1.3:

```json
"tls-min-version": "VersionTLS13"
```

Expected output for TLS 1.2 with explicit ciphers:

```json
"tls-min-version": "VersionTLS12",
"tls-cipher-suites": ["TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", ...]
```

If neither field is present, the Go default (TLS 1.2, Go-selected cipher suites) applies.

## Relationship to other TLS configuration

These variables control the **TLS version and cipher suite policy** for the CAA↔APF connection. They are separate from the certificate and key configuration described in [tls-proxy-forwarder.md](tls-proxy-forwarder.md), which controls *which* certificates are used for mutual authentication. Both sets of configuration can be used together.
