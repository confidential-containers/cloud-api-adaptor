# Initdata

The document describes the implementation of the [initdata](https://github.com/confidential-containers/trustee/blob/main/kbs/docs/initdata.md) spec in PeerPods.

Initdata is used when `AA_KBC_PARAMS` is not set at the moment, the plan is to remove `AA_KBC_PARAMS` support after `initdata` function works completely.

## Initdata example

[attestation-agent](https://github.com/confidential-containers/guest-components/tree/main/attestation-agent) config file `aa.toml`, [confidential-data-hub](https://github.com/confidential-containers/guest-components/tree/main/confidential-data-hub) config file `cdh.toml` and a lightweight policy file `polciy.rego` can be passed into PeerPod via initdata.

Example:
```toml
algorithm = "sha384"
version = "0.1.0"

[data]
"aa.toml" = '''
[token_configs]
[token_configs.coco_as]
url = 'http://127.0.0.1:8080'

[token_configs.kbs]
url = 'http://127.0.0.1:8080'
'''

"cdh.toml"  = '''
socket = 'unix:///run/confidential-containers/cdh.sock'
credentials = []

[kbc]
name = 'cc_kbc'
url = 'http://1.2.3.4:8080'
'''

"policy.rego" = '''
package agent_policy

import future.keywords.in
import future.keywords.every

import input

# Default values, returned by OPA when rules cannot be evaluated to true.
default CopyFileRequest := false
default CreateContainerRequest := false
default CreateSandboxRequest := true
default DestroySandboxRequest := true
default ExecProcessRequest := false
default GetOOMEventRequest := true
default GuestDetailsRequest := true
default OnlineCPUMemRequest := true
default PullImageRequest := true
default ReadStreamRequest := false
default RemoveContainerRequest := true
default RemoveStaleVirtiofsShareMountsRequest := true
default SignalProcessRequest := true
default StartContainerRequest := true
default StatsContainerRequest := true
default TtyWinResizeRequest := true
default UpdateEphemeralMountsRequest := true
default UpdateInterfaceRequest := true
default UpdateRoutesRequest := true
default WaitProcessRequest := true
default WriteStreamRequest := false
'''
```

## Annotation in Pod yaml
Generate base64 encoded string based on above example and pass it into PeerPod via annotation `io.katacontainers.config.runtime.cc_init_data`:
```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    run: busybox
  name: busybox
  annotations:
    io.katacontainers.config.runtime.cc_init_data: YWxnb3JpdGhtID0gInNoYTM4NCIKdmVyc2lvbiA9ICIwLjEuMCIKCltkYXRhXQoiYWEudG9tbCIgPSAnJycKW3Rva2VuX2NvbmZpZ3NdClt0b2tlbl9jb25maWdzLmNvY29fYXNdCnVybCA9ICdodHRwOi8vMTI3LjAuMC4xOjgwODAnCgpbdG9rZW5fY29uZmlncy5rYnNdCnVybCA9ICdodHRwOi8vMTI3LjAuMC4xOjgwODAnCicnJwoKImNkaC50b21sIiAgPSAnJycKc29ja2V0ID0gJ3VuaXg6Ly8vcnVuL2NvbmZpZGVudGlhbC1jb250YWluZXJzL2NkaC5zb2NrJwpjcmVkZW50aWFscyA9IFtdCgpba2JjXQpuYW1lID0gJ2NjX2tiYycKdXJsID0gJ2h0dHA6Ly8xLjIuMy40OjgwODAnCicnJwoKInBvbGljeS5yZWdvIiA9ICcnJwpwYWNrYWdlIGFnZW50X3BvbGljeQoKaW1wb3J0IGZ1dHVyZS5rZXl3b3Jkcy5pbgppbXBvcnQgZnV0dXJlLmtleXdvcmRzLmV2ZXJ5CgppbXBvcnQgaW5wdXQKCiMgRGVmYXVsdCB2YWx1ZXMsIHJldHVybmVkIGJ5IE9QQSB3aGVuIHJ1bGVzIGNhbm5vdCBiZSBldmFsdWF0ZWQgdG8gdHJ1ZS4KZGVmYXVsdCBDb3B5RmlsZVJlcXVlc3QgOj0gZmFsc2UKZGVmYXVsdCBDcmVhdGVDb250YWluZXJSZXF1ZXN0IDo9IGZhbHNlCmRlZmF1bHQgQ3JlYXRlU2FuZGJveFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IERlc3Ryb3lTYW5kYm94UmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgRXhlY1Byb2Nlc3NSZXF1ZXN0IDo9IGZhbHNlCmRlZmF1bHQgR2V0T09NRXZlbnRSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBHdWVzdERldGFpbHNSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBPbmxpbmVDUFVNZW1SZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBQdWxsSW1hZ2VSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBSZWFkU3RyZWFtUmVxdWVzdCA6PSBmYWxzZQpkZWZhdWx0IFJlbW92ZUNvbnRhaW5lclJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFJlbW92ZVN0YWxlVmlydGlvZnNTaGFyZU1vdW50c1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFNpZ25hbFByb2Nlc3NSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBTdGFydENvbnRhaW5lclJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFN0YXRzQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgVHR5V2luUmVzaXplUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgVXBkYXRlRXBoZW1lcmFsTW91bnRzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgVXBkYXRlSW50ZXJmYWNlUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgVXBkYXRlUm91dGVzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgV2FpdFByb2Nlc3NSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBXcml0ZVN0cmVhbVJlcXVlc3QgOj0gZmFsc2UKJycn
spec:
  containers:
  - image: quay.io/prometheus/busybox
    name: busybox
    resources: {}
  dnsPolicy: ClusterFirst
  restartPolicy: Never
  runtimeClassName: kata-remote
``` 

## Structure in `write_files`
cloud-api-adaptor will read the annotation and write it to [write_files](../../cloud-providers/util/cloudinit/cloudconfig.go). Note: files unrelated to initdata (like network tunnel configuration in `/run/peerpod/daemon.json`) are also part of the `write_files` directive.
```yaml
write_files:
- path: /run/peerpod/agent-config.toml
  content: 
- path: /run/peerpod/daemon.json
  content: 
- path: /run/peerpod/auth.json
  content:
- path: /run/peerpod/initdata
  content:
```

## Provision initdata files.
`/run/peerpod/aa.toml`, `/run/peerpod/cdh.toml` and `/run/peerpod/policy.rego` will be provisioned from `/run/peerpod/initdata` via [process-user-data](../cmd/process-user-data/main.go).

It also calculates the digest `/run/peerpod/initdata.digest` based on the `algorithm` in `/run/peerpod/initdata` and its contents.

`/run/peerpod/initdata.digest` could be used by the TEE drivers.

The digest can be calculated manually and set to attestation service policy before hand if needed. To calculate the digest, use a tool (for example some online sha tools) to calculate the hash value based on the initdata annotation string. The calculated sha384 is: `14980c75860de9adcba2e0e494fc612f0f4fe3d86f5dc8e238a3255acfdf43bf82b9ccfc21da95d639ff0c98cc15e05e` for above sample.

## TODO
A large policy bodies that cannot be provisioned via IMDS user-data, the limitation depends on providers IMDS limitation. We need add checking and limitations according to test result future. 
