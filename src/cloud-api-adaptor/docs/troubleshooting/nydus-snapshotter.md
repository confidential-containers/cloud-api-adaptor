# Some tips for debugging nydus-snapshotter related issues

## Check nydus configuration

When using `containerd` runtime, kata-deploy configures `nydus-snapshotter` to handle in-guest
container image pull.

Login to the worker node and run the following command to verify if `nydus-snapshotter` is set up successfully.

```sh
ctr -a /run/containerd/containerd.sock plugin ls | grep nydus
```

You should see an o/p like the following:

```sh
io.containerd.snapshotter.v1    nydus                    -              ok
```

## Pod creation fails with "error unpacking image"

Sometimes when creating a pod you might encounter the following error:

```sh
Warning  Failed     8m51s (x7 over 10m)  kubelet            Error: failed to create containerd container: error unpacking image: failed to extract layer sha256:d51af96cf93e225825efd484ea457f867cb2b967f7415b9a3b7e65a2f803838a: failed to get reader from content store: content digest sha256:ec562eabd705d25bfea8c8d79e4610775e375524af00552fe871d3338261563c: not found
```

If you encounter this error, first check if the `discard_unpacked_layers`
setting is enabled in the containerd configuration. This setting removes
compressed image layers after unpacking, which can cause issues because
PeerPods workloads rely on those layers. To disable it, update
`/etc/containerd/config.toml` with:

```toml
[plugins."io.containerd.grpc.v1.cri".containerd]
  discard_unpacked_layers = false
```

Restart containerd after making the change:

```bash
sudo systemctl restart containerd
```

Alternatively, you can pre-fetch the images on the worker nodes:

To do this, login to the worker node and explicitly fetch the image using the following command:

```sh
export image=<_set_>
ctr -n k8s.io content fetch $image
```

Here is a concrete example describing how to download the images in case of the above error
when running e2e tests:

```sh
images=(
  "quay.io/prometheus/busybox:latest"
  "quay.io/confidential-containers/test-images:testworkdir"
  "docker.io/library/nginx:latest"
  "docker.io/curlimages/curl:8.4.0"
  "quay.io/curl/curl:latest"
)

# Loop through each image in the list
# and download the image
for image in "${images[@]}"; do
    ctr -n k8s.io content fetch $image
done
```

