# Container Image Registries Authentication

## Using Imagepull Secrets in the pod spec

Even though in the current CoCo configuration the images are being pulled
directly inside the pod vm, the pod spec still needs to have the image pull
secret defined. This is because metadata of an OCI image is still being
accessed on the worker node, even if kata-agent pulls an image in the pod vm.
Please refer to the kubernetes docs to learn more about [image pull
secrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod).

The image pull secret is part of the infrastructure and available to the
(untrusted) worker node.
You must use signed or encrypted container images to protect the container image.

CAA configures kata-agent with auth.json by reading the pod and service account image pull secrets.

## Embedding registry secret in the pod VM image

This is an alternative mechanism where instead of providing the image registry
secret via the pod spec, you create a custom pod vm image and add the [registry authentication
file](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md)
to it.

