## Container Image Registries Authentication

To authenticate with private container image registry you are required to provide
[registry authentication file](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md) to your podvm in order
to allow the image to be pulled directly.

Registry authentication file can be provided either statically or at runtime.


### Statically embed authentication file in podvm image

- `cd ~/cloud-api-adaptor/podvm/files/etc`
- Base64 your [auth.json](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md), this can be done by doing `cat auth.json | base64 -w 0`
- Export the base64 encoded file `export AUTHFILE=<base64-encoded-auth.json>`
- Create and Add the base64 encoded auth file into the `aa-offline_fs_kbc-resources.json` like so:
```
cat <<EOF | tee aa-offline_fs_kbc-resources.json
{
  "Credential": "${AUTHFILE}"
}
EOF
```
- Add `AA_KBC="offline_fs_kbc"` prior to the `make image` step

### Deploy authentication file along with cloud-api-adaptor deployment

- Add `AA_KBC="offline_fs_kbc"` prior to the `make image` image build
- Make sure you set [auth.json](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md) file for the `auth-json-secret`
when you configure `install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml` prior to `make deploy`
