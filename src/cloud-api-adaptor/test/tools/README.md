# Introduction

tools for development and testing.

## provisioner-cli

`provisioner-cli` provides a cli program that leverage the [cluster provisioner package](../provisioner) to create VPC, Subnet and Cluster and other necessary resources, and then install the PeerPods Helm charts to enable the function in the created cluster. Which is also used to upload a VM image to cloud vendor.

### Build provisioner-cli
In the root directory of `test/tools`, run command as below to build the cli program:
```bash
apt-get install libvirt-dev
make all
```

Program is generated: `test/tools/caa-provisioner-cli`.
Optionally, `BUILTIN_CLOUD_PROVIDERS` could also be used to build the CLI for specific providers, like:
```bash
make BUILTIN_CLOUD_PROVIDERS="ibmcloud" all
```

### Use provisioner-cli
In directory `test/tools`, run commands like:
```bash
export TEST_PODVM_IMAGE=${POD_IMAGE_FILE_PATH}
export LOG_LEVEL=${LOG_LEVEL}
export CLOUD_PROVIDER=${CLOUD_PROVIDER}
export TEST_PROVISION_FILE=${PROPERTIES_FILE_PATH}
export TEST_PROVISION="yes"
export DEPLOY_KBS="yes"
export INSTALL_DIR="../../install"
./caa-provisioner-cli -action=${ACTION}
```
`ACTION` supports `provision`, `deprovision`, `install`, `uninstall` and `uploadimage`.

`INSTALL_DIR` needs to be the relative or absolute path to the directory `cloud-api-adaptor/install` e.g. `../../install` if running from `test/tools` directory.

### Brief Action Explanations

`provision` : Uses the provisioner to create a new cluster, and install cloud-api-adaptor resources

`deprovision` : Deletes the cluster that we previously created

`install` : Install the cloud-api-adaptor using the Helm charts to an existing cluster, must set `KUBECONFIG`

`uninstall` : Removes the cloud-api-adaptor resources from the cluster, must set `KUBECONFIG`

### Add a new provider support
`ibmcloud`, `azure` and `libvirt` providers are supported now, to add a new provider please add it in [cluster provisioner package](../provisioner)
