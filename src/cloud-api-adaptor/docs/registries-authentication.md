# Container Image Registries Authentication

To authenticate with private container image registry you are required to provide
[registry authentication file](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md) to your podvm in order to allow the image to be pulled directly.

## Pull Secrets on the Worker Node

Even though in the current CoCo configuration the images are being pulled on the pod, the pod spec still needs to have the pull secret defined. This is because metadata of an OCI image is still being accessed on the worker node, even if kata-agent pulls an image in the podvm. Please refer to the kubernetes docs to learn more about [pull secrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod).

CAA configures kata-agent with auth.json by reading the pod and service account image pull secrets.
