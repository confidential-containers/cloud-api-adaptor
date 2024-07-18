# Container Image Registries Authentication

To authenticate with private container image registry you are required to provide
[registry authentication file](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md) to your podvm in order to allow the image to be pulled directly.

## Pull Secrets on the Worker Node

Even though in the current CoCo configuration the images are being pulled on the pod, the pod spec still needs to have the pull secret defined. This is because metadata of an OCI image is still being accessed on the worker node, even if kata-agent pulls an image in the podvm. Please refer to the kubernetes docs to learn more about [pull secrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod).

## Deploy auth.json along with cloud-api-adaptor deployment

- CAA receives a registry auth file from the `auth-json-secret` secret that is mounted in the CAA pod using `install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml`.
- Make sure you do set a valid [auth.json](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md) as an entry for `auth-json-secret` when you configure `install/overlays/$(CLOUD_PROVIDER)/kustomization.yaml` prior to `make deploy`
- If CAA encounters an auth.json file, it will configure kata-agent to use it.
