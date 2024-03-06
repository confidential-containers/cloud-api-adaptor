# Kata Agent Policy

Agent Policy is a Kata Containers feature that enables the Guest VM to perform additional validation for each agent API request.

Note: For using agent policy with peer-pods, you'll need a kata shim with agent-policy support. 

# Enabling the Kata Agent Policy

The following makefile options are available

- AGENT_POLICY option for building the kata-agent with policy support.
  This is enabled by default

- DEFAULT_AGENT_POLICY_FILE option to specify the policy file to be used as default.
  The default policy file is allow-all.rego


When compiled with default settings, the following happens

1. The [`Open Policy Agent (OPA)`](https://www.openpolicyagent.org/) binary gets built and installed in the VM image.

2. The `kata-opa` service gets included in the VM image

3. A default policy which allows all api is enabled

Two additional policy example files are provided: 
1. allow-all-except-exec-process.rego: This policy disables the `ExecProcess` API, thereby preventing `kubectl exec` against the pod. 
2. disallow-all-except-setpolicy.rego: This policy only enables the `SetPolicy` API. The pod should provide the required policy via annotation.

You can configure the base policy for the VM image by using the `DEFAULT_AGENT_POLICY_FILE` option.

For example, to create the Azure VM image with the default policy as `disallow-all-except-setpolicy.rego`, you
can run the following command

```
cd azure/image
CLOUD_PROVIDER=azure PODVM_DISTRO=ubuntu DEFAULT_AGENT_POLICY_FILE=disallow-all-except-setpolicy.rego make image
```

## Specify Policy as a Kubernetes `YAML` annotation

Kubernetes users can encode in `base64` format their Policy documents, and add the encoded string as an annotation. Example:

### Encode a Policy file

For example, the
[`allow-all-except-exec-process.rego`](../../../podvm/files/etc/kata-opa/allow-all-except-exec-process.rego)
sample policy file is different from the [default
Policy](../../../podvm/files/etc/kata-opa/allow-all.rego) because it rejects any `ExecProcess`
requests. You can encode this policy file:

```bash
$ base64 -w 0 allow-all-except-exec-process.rego
cGFja2FnZSBhZ2VudF9wb2xpY3kKCmRlZmF1bHQgQWRkQVJQTmVpZ2hib3JzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgQWRkU3dhcFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IENsb3NlU3RkaW5SZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBDb3B5RmlsZVJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IENyZWF0ZUNvbnRhaW5lclJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IENyZWF0ZVNhbmRib3hSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBEZXN0cm95U2FuZGJveFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IEdldE1ldHJpY3NSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBHZXRPT01FdmVudFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IEd1ZXN0RGV0YWlsc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IExpc3RJbnRlcmZhY2VzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgTGlzdFJvdXRlc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IE1lbUhvdHBsdWdCeVByb2JlUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgT25saW5lQ1BVTWVtUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUGF1c2VDb250YWluZXJSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBQdWxsSW1hZ2VSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBSZWFkU3RyZWFtUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVtb3ZlQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVtb3ZlU3RhbGVWaXJ0aW9mc1NoYXJlTW91bnRzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVzZWVkUmFuZG9tRGV2UmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVzdW1lQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgU2V0R3Vlc3REYXRlVGltZVJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFNldFBvbGljeVJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFNpZ25hbFByb2Nlc3NSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBTdGFydENvbnRhaW5lclJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFN0YXJ0VHJhY2luZ1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFN0YXRzQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgU3RvcFRyYWNpbmdSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBUdHlXaW5SZXNpemVSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVDb250YWluZXJSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVFcGhlbWVyYWxNb3VudHNSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVJbnRlcmZhY2VSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVSb3V0ZXNSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBXYWl0UHJvY2Vzc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFdyaXRlU3RyZWFtUmVxdWVzdCA6PSB0cnVlCgpkZWZhdWx0IEV4ZWNQcm9jZXNzUmVxdWVzdCA6PSBmYWxzZQo=
```

### Attach the Policy to a pod

Add the encoded Policy to your `YAML` file - e.g., `pod1.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: policy-exec-rejected
  annotations:
    io.katacontainers.config.agent.policy: cGFja2FnZSBhZ2VudF9wb2xpY3kKCmRlZmF1bHQgQWRkQVJQTmVpZ2hib3JzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgQWRkU3dhcFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IENsb3NlU3RkaW5SZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBDb3B5RmlsZVJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IENyZWF0ZUNvbnRhaW5lclJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IENyZWF0ZVNhbmRib3hSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBEZXN0cm95U2FuZGJveFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IEdldE1ldHJpY3NSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBHZXRPT01FdmVudFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IEd1ZXN0RGV0YWlsc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IExpc3RJbnRlcmZhY2VzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgTGlzdFJvdXRlc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IE1lbUhvdHBsdWdCeVByb2JlUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgT25saW5lQ1BVTWVtUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUGF1c2VDb250YWluZXJSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBQdWxsSW1hZ2VSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBSZWFkU3RyZWFtUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVtb3ZlQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVtb3ZlU3RhbGVWaXJ0aW9mc1NoYXJlTW91bnRzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVzZWVkUmFuZG9tRGV2UmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVzdW1lQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgU2V0R3Vlc3REYXRlVGltZVJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFNldFBvbGljeVJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFNpZ25hbFByb2Nlc3NSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBTdGFydENvbnRhaW5lclJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFN0YXJ0VHJhY2luZ1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFN0YXRzQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgU3RvcFRyYWNpbmdSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBUdHlXaW5SZXNpemVSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVDb250YWluZXJSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVFcGhlbWVyYWxNb3VudHNSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVJbnRlcmZhY2VSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVSb3V0ZXNSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBXYWl0UHJvY2Vzc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFdyaXRlU3RyZWFtUmVxdWVzdCA6PSB0cnVlCgpkZWZhdWx0IEV4ZWNQcm9jZXNzUmVxdWVzdCA6PSBmYWxzZQo=
    io.containerd.cri.runtime-handler: kata-remote 
spec:
  runtimeClassName: kata-remote
  containers:
    - name: first-test-container
      image: quay.io/prometheus/busybox:latest
      command:
        - sleep
        - "120"
```

Create the pod:

```bash
$ kubectl apply -f pod1.yaml
```

While creating the Pod sandbox, the Kata Shim will notice the
`io.katacontainers.config.agent.policy` annotation and will send the Policy
document to the Kata Agent - by sending a `SetPolicy` request. Note that this
request will fail if the default Policy, included in the Guest image, doesn't
allow this `SetPolicy` request. If the `SetPolicy` request is rejected by the
Guest, the Kata Shim will fail to start the Pod sandbox.
