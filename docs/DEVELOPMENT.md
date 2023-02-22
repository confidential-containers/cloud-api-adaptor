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

## Building the binaries

Running `make` builds two binaries.
- cloud-api-adaptor: This is the main program responsibile for Pod VM lifecycle management. 
By default this is built as a dynamically linked binary with support for libvirt provider.

- agent-protocol-forwarder: This is the program which runs inside the Pod VM to forward the
kata-agent protocol over TCP. This is a statically linked binary and is common for all the providers.

### Release and Dev builds

This is controlled by the variable `RELEASE_BUILD`. By default this is set to `false` and builds 
all the providers into the cloud-api-adaptor binary.

Further, note that the dev build is a dynamically linked binary as it includes the `libvirt` provider.

The libvirt go library uses `cgo` and hence when `libvirt` provider is included there is no statically linked
binary.

Consequently you'll need to run the binary on the same OS/version where you have built it.

You'll also need to install the libvirt dev packages before running the build.

Example, if you are using Ubuntu then run the following command
```
sudo apt-get install -y libvirt-dev
```
The resultant `cloud-api-adaptor` binary will include support for all the providers including `libvirt`.

To create release build which doesn't include the `libvirt` provider and creates a statically linked
binary, run the following command:
```
RELEASE_BUILD=true make
```
The resultant `cloud-api-adaptor` binary will include support for all the providers excluding `libvirt`.

### Build only specific providers

You can also build specific providers as per your requirement.

For example, run the following command to build only the `aws` provider:
```
GOFLAGS="-tags=aws" make
```

For example, run the following command to build the `aws`, `azure` and `ibmcloud` providers:
```
GOFLAGS="-tags=aws,azure,ibmcloud" make
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
