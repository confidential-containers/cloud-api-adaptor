## Prerequisites
- make
- golang 1.18.y
- g++

## Clone the source code
```
export BUILD_DIR=$HOME/remote-hyp
mkdir -p $BUILD_DIR && cd $BUILD_DIR

git clone -b CCv0 https://github.com/kata-containers/kata-containers.git
git clone https://github.com/confidential-containers/cloud-api-adaptor.git
cd cloud-api-adaptor
```

## Build the binary

Set `CLOUD_PROVIDER` variable to either `aws|azure|ibmcloud|libvirt` depending on your requirement.

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

## Build Kata runtime and agent

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

Build the agent

```
cd $BUILD_DIR/kata-containers/src/agent
make
```
