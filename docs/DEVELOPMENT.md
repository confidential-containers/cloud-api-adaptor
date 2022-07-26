## Prerequisites
- make
- golang 1.16+

## Clone the source code
```
export BUILD_DIR=$HOME/remote-hyp
mkdir -p $BUILD_DIR && cd $BUILD_DIR

git clone -b CCv0-peerpod https://github.com/yoheiueda/kata-containers.git
git clone https://github.com/confidential-containers/cloud-api-adaptor.git
cd cloud-api-adaptor
```

## Build the cloud-api-adaptor binary

Set `CLOUD_PROVIDER` variable to either `aws|ibmcloud|libvirt` depending on your requirement.

```
export CLOUD_PROVIDER=aws
make
```

Note that libvirt go library uses `cgo` and hence there is no static build.

Consequently you'll need to run the binary on the same OS/version where you have
built it.
You'll also need to install the libvirt dev packages before running the build.

Example, if you are using Ubuntu then run the following command
```
sudo apt-get install -y libvirt-dev
```

## Building cloud-api-adaptor container image and deploy it in a pod

Set these variables before performing any of pod/image related operations:
```
export CLOUD_PROVIDER=<aws|ibmcloud|libvirt>
export IMAGE_TAG=quay.io/<example>/cloud-api-adaptor:latest
```
* `make image` builds the container image under the `$IMAGE_TAG` tag
	* currently the container image is not cloud-provider specific and would work with any cloud-provider regardless to `$CLOUD_PROVIDER`
* `make push` pushes the image to your private repo according to the set `$IMAGE_TAG`
* `make deploy` deploys the cloud-api-adaptor pod in the configured cluster
	* provide the `$CLOUD_PROVIDER` specific ConfigMaps and Secrets under `$CLOUD_PROVIDER/deploy/`
	* validate kubectl is available in your `$PATH` and `$KUBECONFIG` is set
* `make delete` deletes the pod deployment from the configured cluster

## Build Kata runtime

Install the prerequisites as mentioned in the following [link](https://github.com/kata-containers/kata-containers/blob/main/docs/Developer-Guide.md#requirements-to-build-individual-components)

Install `protoc`
```
wget -c https://github.com/protocolbuffers/protobuf/releases/download/v3.11.4/protoc-3.11.4-linux-x86_64.zip
sudo unzip protoc-3.11.4-linux-x86_64.zip -d /usr/local
```

Build the runtime

```
cd $BUILD_DIR/kata-containers/src/runtime
make
```
