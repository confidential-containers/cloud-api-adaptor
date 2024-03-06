## Container Image Registries Authentication

To authenticate with private container image registry you are required to provide
[registry authentication file](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md) to your podvm in order
to allow the image to be pulled directly.

Registry authentication file can be provided either statically or at runtime.

### Understanding workflow

- For pulling images from authenticated registries you need the [**attestation-agent**](https://github.com/confidential-containers/guest-components/tree/main/attestation-agent) in the podvm. The role of the attestation-agent is to provide the registry credentials to the `kata-agent`. The podvm image that you are using should be built with `AA_KBC="offline_fs_kbc`. This ensures that [agent-config.toml](../podvm/files/etc/agent-config.toml) in podVM should have `aa_kbc_params = "offline_fs_kbc::null"` set.
- The registry credentials also need to be available in a file inside the Pod VM image. The config `aa_kbc_params = "offline_fs_kbc::null` tells the attestation-agent to retrieve secrets from the **local filesystem**. The registry credentials are embedded in a resources file on a fixed path on the local filesystem: `/etc/aa-offline_fs_kbc-resources.json`.

### Statically embed authentication file in podvm image

- `cd ~/cloud-api-adaptor/podvm/files/etc`
- Base64 your [auth.json](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md), this can be done by doing `cat auth.json | base64 -w 0`
- Export the base64 encoded file `export AUTHFILE=<base64-encoded-auth.json>`
- Create and Add the base64 encoded auth file into the `aa-offline_fs_kbc-resources.json` like so:
```
cat <<EOF | tee aa-offline_fs_kbc-resources.json
{
  "default/credential/test": "${AUTHFILE}"
}
EOF
```
- **Important:** Make sure to build image with `AA_KBC="offline_fs_kbc" make image`.

### Deploy authentication file along with cloud-api-adaptor deployment

- The cloud-api-adaptor (CAA) provides the secret to the local fs in the podvm image by attaching it. This secret is converted and copied using `cloud-init` to `/etc/aa-offline_fs_kbc-resources.json` on the podvm.
- CAA gets the secret from the auth-json-secret secret that is mounted inside the CAA pod using `install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml`.
- **Important:** Make sure to build image with `AA_KBC="offline_fs_kbc" make image`.
- Make sure you set [auth.json](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md) file for the `auth-json-secret`
when you configure `install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml` prior to `make deploy`
