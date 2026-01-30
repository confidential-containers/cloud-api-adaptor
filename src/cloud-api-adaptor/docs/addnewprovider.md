# :memo: Adding support for a new built-in provider

### Step 1: Initialize and register the cloud provider manager

The provider-specific cloud manager should be placed under `src/cloud-providers/<provider>`.

:information_source:[Example code](../../cloud-providers/aws)

### Step 2: Add provider specific code

Under `src/cloud-providers/<provider>`, start by adding a new file called `types.go`. This file defines a configuration struct that contains the required parameters for a cloud provider.

:information_source:[Example code](../../cloud-providers/aws/types.go)

#### Step 2.1: Implement the Cloud interface

Create a provider-specific manager file called `manager.go`, which implements the following methods for parsing command-line flags, loading environment variables, and creating a new provider.

- ParseCmd
- LoadEnv
- NewProvider

Create an `init` function to add your manager to the cloud provider table.

```go
func init() {
 cloud.AddCloud("aws", &Manager{})
}
```

:information_source:[Example code](../../cloud-providers/aws/manager.go)

#### Step 2.2: Implement the Provider interface

The Provider interface defines a set of methods that need to be implemented by the cloud provider for managing virtual instances. Add the required methods:

- CreateInstance
- DeleteInstance
- Teardown

:information_source:[Example code](../../cloud-providers/aws/provider.go)

Also, consider adding additional files to modularize the code. You can refer to existing providers such as `aws`, `azure`, `ibmcloud`, and `libvirt` for guidance. Adding unit tests wherever necessary is good practice.

#### Step 2.3: Include Provider package from main

To include your provider you need reference it from the main package. Go build tags are used to selectively include different providers.

:information_source:[Example code](../../cloud-api-adaptor/cmd/cloud-api-adaptor/aws.go)

```go
//go:build aws
```

Note the comment at the top of the file, when building ensure `-tags=` is set to include your new provider. See the [Makefile](../../cloud-api-adaptor/Makefile#L26) for more context and usage.

### Step 2.4 Add code to receive user-data on the Pod VM image

A Pod VM image is configured via user-data that is provided to the guest. How the guest retrieves the user-data body is specific to the provider. Add support for the provider to the [userdata module](../pkg/userdata/provision.go)

### Step 3: Add documentation on how to build a Pod VM image

For using the provider, a pod VM image needs to be created in order to create the peer pod instances. Add the instructions for building the peer pod VM image at the root directory similar to the other providers.

### Step 4: Add E2E tests for the new provider

For more information, please refer to the section on [adding support for a new cloud provider](../test/e2e/README.md#adding-support-for-a-new-cloud-provider) in the E2E testing documentation.

# :memo: Adding support for a new external provider

External plugins are loaded dynamically and you don't need to recompile `cloud-api-adaptor` and `peerpod-ctrl` for adding external plugins.

The following section describes building and using an external `libvirt` plugin.

Assume you are using the `cloud-api-adaptor` and `peerpod-ctrl` image with the external plugin function support.

And you are currently located in the root folder of the cloud-api-adaptor on your development machine.

### Step 1: Initialize and register the cloud provider manager

```bash
mkdir -p src/libvirt/build
cd src/libvirt

cat > manager.go <<EOF
//go:build cgo
package main

import (
 "flag"

 providers "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
 "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/libvirt"
)

type Manager struct {
 libvirtManager *libvirt.Manager
}

func init() {
 libvirtManager := &libvirt.Manager{}
 manager := &Manager{
  libvirtManager: libvirtManager,
 }
 providers.AddCloudProvider("libvirt", manager)
}

func (m *Manager) ParseCmd(flags *flag.FlagSet) {
 m.libvirtManager.ParseCmd(flags)
}

// DEPRECATED: LoadEnv() is deprecated and will be removed in a future release.
// Environment variables are now loaded during ParseCmd() via FlagRegistrar.
// For compatibility, this method must still exist but should be a no-op.
func (m *Manager) LoadEnv() {
 // No longer needed - environment variables are handled in ParseCmd
}

func (m *Manager) NewProvider() (providers.Provider, error) {
 return NewProvider(m.libvirtManager.GetConfig())
}
EOF
```

> **Note:** The the package name must be "main" for external plugin, all other required methods are same as built-in plugin.

### Step 2: Add provider specific code

```bash
cat > provider.go <<EOF
//go:build cgo

package main

import (
 "context"
 "fmt"
 "log"

 providers "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
 "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/libvirt"
 "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/libvirt] ", log.LstdFlags|log.Lmsgprefix)

type libvirtext struct {
 libvirtProvider providers.Provider
 serviceConfig   *libvirt.Config
}

func NewProvider(config *libvirt.Config) (providers.Provider, error) {

 libvirtProvider, err := libvirt.NewProvider(config)

 if err != nil {
  return nil, err
 }

 provider := &libvirtext{
  libvirtProvider: libvirtProvider,
  serviceConfig:   config,
 }

 return provider, nil
}

func (p *libvirtext) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec providers.InstanceTypeSpec) (*providers.Instance, error) {
 cloudInitCloudConfigData, ok := cloudConfig.(*cloudinit.CloudConfig)
 // Debug print cloudInitCloudConfigData
 if !ok {
  return nil, fmt.Errorf("User Data generator did not use the cloud-init Cloud Config data format")
 }
 userData, err := cloudInitCloudConfigData.Generate()
 if err != nil {
  return nil, err
 }
 logger.Printf("===CreateInstance: userData from libvirt: %s", userData)

 return p.libvirtProvider.CreateInstance(ctx, podName, sandboxID, cloudConfig, spec)
}

func (p *libvirtext) DeleteInstance(ctx context.Context, instanceID string) error {
 return p.libvirtProvider.DeleteInstance(ctx, instanceID)
}

func (p *libvirtext) Teardown() error {
 return nil
}

func (p *libvirtext) ConfigVerifier() error {
 VolName := p.serviceConfig.VolName
 if len(VolName) == 0 {
  return fmt.Errorf("VolName is empty")
 }
 return nil
}
EOF
```

### Step 3: Prepare the go.mod and go.sum

```bash
cat > go.mod <<EOF
module github.com/confidential-containers/cloud-api-adaptor/src/libvirt

go 1.20

require github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers v0.8.2

replace github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers => ../cloud-providers

EOF

go mod tidy
```

### Step 4: build the external cloud provider plugin file via docker

```bash
cat > Dockerfile <<EOF
ARG BUILDER_BASE=quay.io/confidential-containers/golang-fedora:1.24.12-43
FROM --platform="\$TARGETPLATFORM" \$BUILDER_BASE AS builder
RUN dnf install -y libvirt-devel && dnf clean all
WORKDIR /work
COPY ./cloud-providers ./cloud-providers
COPY ./libvirt ./libvirt

WORKDIR /work/libvirt
RUN GOOS=linux GOARCH=\$TARGETPLATFORM CGO_ENABLED=1 go build -tags=libvirt -buildmode=plugin .

FROM --platform=\$TARGETPLATFORM alpine:latest
COPY --from=builder /work/libvirt/libvirt.so /libvirt.so
EOF

cd ../ && docker buildx build --platform "linux/amd64" \
 -t quay.io/confidential-containers/libvirt \
 -f libvirt/Dockerfile \
 -o type=local,dest="./libvirt/build" \
 .
cd ../
```

> **Note:** the external cloud provider plugin need to be built using the same golang and package versions that was used to build cloud-api-adaptor.

The built out external plugin file is "src/libvirt/build/libvirt.so"

### Step 5: Prepare the test libvirt peerpod env by following this [document](../libvirt/README.md)

### Step 6: Update the "peer-pods-cm" configmap to enable cloud provider external `libvirt` plugin

- Calculate the SHA256 checksum of the built external plugin

```bash
sha256sum src/libvirt/build/libvirt.so
60e5cdbcb910c6331c796ce66dfa32e50bf083689ffdf18ee136d91a9da5ddab src/libvirt/build/libvirt.so
```

- Update "peer-pods-cm" configmap

```bash
kubectl edit cm peer-pods-cm -n confidential-containers-system
...
  CLOUD_PROVIDER: libvirt
  ENABLE_CLOUD_PROVIDER_EXTERNAL_PLUGIN: "true"
  CLOUD_PROVIDER_EXTERNAL_PLUGIN_HASH: 60e5cdbcb910c6331c796ce66dfa32e50bf083689ffdf18ee136d91a9da5ddab
  CLOUD_PROVIDER_EXTERNAL_PLUGIN_PATH: /cloud-providers/libvirt.so
...
```

> **Note:** CLOUD_PROVIDER_EXTERNAL_PLUGIN_HASH is sha256sum of the built out external cloud provider plugin

### Step 7: Actions on the worker node

- Copy the external plugin file to worker node `/opt/cloud-api-adaptor/plugins` folder

```bash
ssh root@worker-ip 'mkdir -p /opt/cloud-api-adaptor/plugins && chmod +x /opt/cloud-api-adaptor/plugins'
scp src/libvirt/build/libvirt.so root@worker-ip:/opt/cloud-api-adaptor/plugins
```

- Prepare `entrypoint.sh` for the external `libvirt` plugin

```bash
cat <<'EOF' > src/libvirt/build/entrypoint.sh
#!/bin/bash

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER}
# Enabling dynamically loaded cloud provider external plugin feature, disabled by default
ENABLE_CLOUD_PROVIDER_EXTERNAL_PLUGIN=${ENABLE_CLOUD_PROVIDER_EXTERNAL_PLUGIN:-false}

CRI_RUNTIME_ENDPOINT=${CRI_RUNTIME_ENDPOINT:-/run/cri-runtime.sock}
optionals+=""

# Ensure you add a space before the closing quote (") when updating the optionals
# example:
# following is the correct method: optionals+="-option val "
# following is the incorrect method: optionals+="-option val"

[[ "${PAUSE_IMAGE}" ]] && optionals+="-pause-image ${PAUSE_IMAGE} "
[[ "${TUNNEL_TYPE}" ]] && optionals+="-tunnel-type ${TUNNEL_TYPE} "
[[ "${VXLAN_PORT}" ]] && optionals+="-vxlan-port ${VXLAN_PORT} "
[[ "${CACERT_FILE}" ]] && optionals+="-ca-cert-file ${CACERT_FILE} "
[[ "${CERT_FILE}" ]] && [[ "${CERT_KEY}" ]] && optionals+="-cert-file ${CERT_FILE} -cert-key ${CERT_KEY} "
[[ "${TLS_SKIP_VERIFY}" ]] && optionals+="-tls-skip-verify "
[[ "${PROXY_TIMEOUT}" ]] && optionals+="-proxy-timeout ${PROXY_TIMEOUT} "
[[ "${INITDATA}" ]] && optionals+="-initdata ${INITDATA} "
[[ "${FORWARDER_PORT}" ]] && optionals+="-forwarder-port ${FORWARDER_PORT} "
[[ "${CLOUD_CONFIG_VERIFY}" == "true" ]] && optionals+="-cloud-config-verify "

test_vars() {
    for i in "$@"; do
        [ -z "${!i}" ] && echo "\$$i is NOT set" && EXT=1
    done
    [[ -n $EXT ]] && exit 1
}

one_of() {
    for i in "$@"; do
        [ -n "${!i}" ] && echo "\$$i is SET" && EXIST=1
    done
    [[ -z $EXIST ]] && echo "At least one of these must be SET: $*" && exit 1
}

libvirt() {
    test_vars LIBVIRT_URI

    [[ "${DISABLECVM}" = "true" ]] && optionals+="-disable-cvm "
    set -x
    exec cloud-api-adaptor libvirt \
        -uri "${LIBVIRT_URI}" \
        -data-dir /opt/data-dir \
        -pods-dir /run/peerpod/pods \
        -network-name "${LIBVIRT_NET:-default}" \
        -pool-name "${LIBVIRT_POOL:-default}" \
        ${optionals} \
        -socket /run/peerpod/hypervisor.sock
}

help_msg() {
    cat <<'HELP_EOF'
Usage:
 CLOUD_PROVIDER=libvirt $0
or
 $0 libvirt
in addition all cloud provider specific env variables must be set and valid
(CLOUD_PROVIDER is currently set to "$CLOUD_PROVIDER")
HELP_EOF
}

if [[ "$CLOUD_PROVIDER" == "libvirt" ]]; then
    libvirt
else
    help_msg
fi
EOF
chmod +x src/libvirt/build/entrypoint.sh
```

- Copy the external plugin file to worker node `/opt/cloud-api-adaptor/plugins` folder

```bash
scp src/libvirt/build/entrypoint.sh root@worker-ip:/opt/cloud-api-adaptor/plugins
```

### Step 8: Update cloud-api-adaptor damonset to use the external `libvirt` plugin

- Run the `kubectl edit` command to update cloud-api-adaptor damonset

```bash
kubectl edit ds cloud-api-adaptor-daemonset -n confidential-containers-system
```

- Overwrite the command

```yaml
    spec:
      containers:
      - command:
        - /cloud-providers/entrypoint.sh
```

- Mount `/opt/cloud-api-adaptor/plugins/` from worker node to the `cloud-api-adaptor-con` container

```yaml
...
        volumeMounts:
        - mountPath: /cloud-providers
          name: provider-dir
...
      volumes:
      - hostPath:
          path: /opt/cloud-api-adaptor/plugins
          type: Directory
        name: provider-dir
...
```

### Step 9: Update peerpod-ctrl deployment to use the external `libvirt` plugin

- Run the edit command to update peerpod-ctrl deployment

```bash
kubectl edit deployment peerpod-ctrl-controller-manager -n confidential-containers-system
```

- Mount `/opt/cloud-api-adaptor/plugins` from worker node to the `manager` container

```yaml
...
        volumeMounts:
        - mountPath: /cloud-providers
          name: provider-dir
...
      volumes:
      - hostPath:
          path: /opt/cloud-api-adaptor/plugins
          type: Directory
        name: provider-dir
...
```

### Step 10: Verify cloud-api-adaptor/peerpod-ctrl pod is running without error

```bash
kubectl logs -n confidential-containers-system ds/cloud-api-adaptor-daemonset

+ exec cloud-api-adaptor libvirt -uri 'qemu+ssh://root@192.168.122.1/system?no_verify=1' -data-dir /opt/data-dir -pods-dir /run/peerpod/pods -network-name default -pool-name default -disable-cvm -socket /run/peerpod/hypervisor.sock
2024/04/17 04:34:56 [adaptor/cloud] Loading external plugin libvirt from /cloud-providers/libvirt.so
2024/04/17 04:34:56 [adaptor/cloud] Successfully opened the external plugin /cloud-providers/libvirt.so
cloud-api-adaptor version v0.8.2-dev
  commit: a8b81333ccb6b0e0adf71c8eda675da97d24d649
  go: go1.21.9
cloud-api-adaptor: starting Cloud API Adaptor daemon for "libvirt"
2024/04/17 04:34:56 [adaptor/cloud/libvirt] libvirt config: &libvirt.Config{URI:"qemu+ssh://root@192.168.122.1/system?no_verify=1", PoolName:"default", NetworkName:"default", DataDir:"/opt/data-dir", DisableCVM:true, VolName:"podvm-base.qcow2", LaunchSecurity:"", Firmware:"/usr/share/edk2/ovmf/OVMF_CODE.fd"}
2024/04/17 04:34:56 [adaptor/cloud/libvirt] Created libvirt connection
2024/04/17 04:34:56 [adaptor] server config: &adaptor.ServerConfig{TLSConfig:(*tlsutil.TLSConfig)(0xc0000d4080), SocketPath:"/run/peerpod/hypervisor.sock", CriSocketPath:"", PauseImage:"", PodsDir:"/run/peerpod/pods", ForwarderPort:"15150", ProxyTimeout:300000000000, EnableCloudConfigVerify:false}
2024/04/17 04:34:56 [util/k8sops] initialized PeerPodService
2024/04/17 04:34:56 [probe/probe] Using port: 8000
2024/04/17 04:34:56 [adaptor] server started
2024/04/17 04:35:35 [probe/probe] nodeName: peer-pods-worker-0
2024/04/17 04:35:35 [probe/probe] Selected pods count: 10
2024/04/17 04:35:35 [probe/probe] Ignored standard pod: kube-proxy-464dj
2024/04/17 04:35:35 [probe/probe] All PeerPods standup. we do not check the PeerPods status any more.
...

kubectl logs -n confidential-containers-system $(kubectl get po -A | grep peerpod-ctrl-controller-manager | awk '{print $2}')

2024-04-17T04:35:20Z INFO controller-runtime.metrics Metrics server is starting to listen {"addr": "127.0.0.1:8080"}
2024/04/17 04:35:20 [adaptor/cloud] Loading external plugin libvirt from /cloud-providers/libvirt.so
2024/04/17 04:35:20 [adaptor/cloud] Successfully opened the external plugin /cloud-providers/libvirt.so
2024/04/17 04:35:20 [adaptor/cloud/libvirt] libvirt config: &libvirt.Config{URI:"qemu+ssh://root@192.168.122.1/system?no_verify=1", PoolName:"default", NetworkName:"default", DataDir:"", DisableCVM:false, VolName:"podvm-base.qcow2", LaunchSecurity:"", Firmware:"/usr/share/edk2/ovmf/OVMF_CODE.fd"}
2024/04/17 04:35:20 [adaptor/cloud/libvirt] Created libvirt connection
2024-04-17T04:35:20Z INFO setup starting manager
2024-04-17T04:35:20Z INFO Starting server {"path": "/metrics", "kind": "metrics", "addr": "127.0.0.1:8080"}
I0417 04:35:20.876671       1 leaderelection.go:248] attempting to acquire leader lease confidential-containers-system/33f6c5d6.confidentialcontainers.org...
2024-04-17T04:35:20Z INFO Starting server {"kind": "health probe", "addr": "[::]:8081"}
I0417 04:35:37.265021       1 leaderelection.go:258] successfully acquired lease confidential-containers-system/33f6c5d6.confidentialcontainers.org
2024-04-17T04:35:37Z DEBUG events peerpod-ctrl-controller-manager-865cb874d-mknth_da8a80e2-4984-4720-828e-3d3b3ff53b2a became leader {"type": "Normal", "object": {"kind":"Lease","namespace":"confidential-containers-system","name":"33f6c5d6.confidentialcontainers.org","uid":"3e18f493-803b-490d-b9e0-b23104bac54e","apiVersion":"coordination.k8s.io/v1","resourceVersion":"17873"}, "reason": "LeaderElection"}
2024-04-17T04:35:37Z INFO Starting EventSource {"controller": "peerpod", "controllerGroup": "confidentialcontainers.org", "controllerKind": "PeerPod", "source": "kind source: *v1alpha1.PeerPod"}
2024-04-17T04:35:37Z INFO Starting Controller {"controller": "peerpod", "controllerGroup": "confidentialcontainers.org", "controllerKind": "PeerPod"}
2024-04-17T04:35:37Z INFO Starting workers {"controller": "peerpod", "controllerGroup": "confidentialcontainers.org", "controllerKind": "PeerPod", "worker count": 1}
...
```

#### Troubleshooting

- "failed to map segment from shared object" from CAA/Peerpod-ctrl log
>
> - Please make sure `CLOUD_PROVIDER_EXTERNAL_PLUGIN_PATH` on worker node have execute permissions, `chmod +x $CLOUD_PROVIDER_EXTERNAL_PLUGIN_PATH`
>
- "plugin was built with a different version of package XXX" from CAA/Peerpod-ctrl log
>
> - Please check the go.mod of CAA and plugins project, the CAA and plugins should be built with same version of issue package XXX
> - Please make sure use same golang env to build CAA, Peerpod-ctrl and cloud-provider plugins
