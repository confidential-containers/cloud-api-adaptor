# Kata Agent Policy

Agent Policy is a Kata Containers feature that enables the Guest VM to perform additional validation for each agent API request.

For a full guide on policy format, how enforcement works, and how to apply policies to pods, see the upstream kata-containers documentation:
https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-use-the-kata-agent-policy.md

## CAA-specific build options

The following makefile option is available when building CAA pod VM images:

- `DEFAULT_AGENT_POLICY_FILE` — specify the policy file to embed as the default policy in the VM image.
  The default is `allow-all.rego`.

For example, to build the pod VM image with the default policy set to `disallow-all-except-setpolicy.rego`:

```
cd podvm
DEFAULT_AGENT_POLICY_FILE=disallow-all-except-setpolicy.rego make image
```
