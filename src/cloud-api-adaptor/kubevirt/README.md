This document explains how to build and deploy Confidential Containers (CoCo) Cloud-API-Adaptor (CAA) for a KubeVirt cluster, and how to run a sample Pod using a Peer Pod VM.

> **Note**: CAA configured with the KubeVirt provider is intended for general KubeVirt clusters.  
Because KubeVirt is a highly customizable cloud platform, additional customization may be required depending on the KubeVirt distribution you use.

## Pod VM Image

Use an existing Pod VM image, or build your own Pod VM image.  
Pod VM images can be pulled from public registries such as `quay.io`.  
For example, you can use `quay.io/confidential-containers/podvm-generic-ubuntu-<ARCH>`, where `<ARCH>` represents your KubeVirt cluster architecture (for example, `amd64` or `arm64`).  
If you are retrieving it from the public registry, you will also need the name of the qcow2 file within the image.  
Please follow the steps below to identify the filename.  
You will need a tool capable of handling containers; in this document, we will use Docker as an example.

1. Pull the image  
    Pull the image you want to use.

    ```bash
    docker pull quay.io/confidential-containers/podvm-generic-ubuntu-<ARCH>:<Tag>
    ```

2. Save the image  
    Save the pulled image as a tar file.

    ```bash
    mkdir <tmpdir>
    cd <tmpdir>
    docker save quay.io/confidential-containers/podvm-generic-ubuntu-<ARCH>:<Tag> > saved_image.tar
    ```

3. Extract the created file  
    Extract the created tar file.  
    After extraction, please extract the largest file located under the `blobs/sha256/` directory.  
    The filename of the extracted file will be the name of the qcow2 file.  
    Please make a note of this, as it will be used in the following steps.  

    ```bash
    tar -xvf saved_image.tar
    cd blobs/sha256
    tar -xvzf <tarfile>
    ```

You can also build your own Pod VM image by following the steps in [Pod VM README](../podvm/README.md).  
After building the image, push it to any Docker registry.  
Keep a note of the registry URL, image name, tag, and qcow2 file name, as they are required in later steps.

## KubeVirt Cluster

> **Note**: This document does not cover how to set up a KubeVirt cluster. For cluster setup instructions, refer to the official KubeVirt documentation.

Prepare authentication credentials to access the KubeVirt cluster.  
CAA requires credentials to authenticate with and communicate to the KubeVirt cluster.  
Prepare the credentials required to access your Kubernetes cluster.  
In most cases, this is `~/.kube/config`.

## CAA Image

A CAA image is a container image that includes CAA components.  
You can either use a prebuilt CAA image or build your own from source code.  
If you choose to use a custom-built CAA image, follow the steps below to build it and push it to any Docker registry.

### Create CAA Image

This section is required only if you build your own CAA image.  
If you use a prebuilt CAA image, you can skip this section.

1.  Install and configure required tools and packages  
    Building and pushing a CAA image requires several tools and packages. Install and configure the following:

    -   docker-engine or podman
    -   make
    -   yq
    -   golang

2.  Clone the CAA source tree
    Clone the CAA source tree from the official repository.

    ```bash
    cd <path-to-working-directory>
    git clone <repository-url> -b <branch-name>
    cd cloud-api-adaptor/src/cloud-api-adaptor
    ```

3.  Build and push the CAA image
    The `Makefile` defines a `make` target for building and pushing the CAA image.  
    Running `make` builds the CAA image and pushes it to the specified Docker registry.  
    The default image tags are set to "latest" and "dev-<VERSION>".  
    "COMMIT" must be specified in `vN.N.N` format.  
    If "COMMIT" is not set, the version tag assigned to the local repository is used.

    ```bash
    export CLOUD_PROVIDER=kubevirt
    export registry=<registry-url>
    export COMMIT=<version tag>
    export VERSION=<image tag>
    make image
    ```
    
### Deploy CAA

Use the provided Helm chart to deploy CAA to a Kubernetes cluster.  
This section assumes your Kubernetes cluster is already set up and that you have permission to deploy applications to that cluster.

1.  Install and configure required tools and packages  
    Deploying a CAA image requires several tools and packages.  
    Install and configure the following:

    -   make
    -   yq
    -   golang
    -   helm

2.  Configure the Helm chart for the KubeVirt provider
    Edit the CAA configuration files to set Kubernetes cluster credentials and the conditions required to start a Peer Pod VM.  
    The KubeVirt provider configuration files are located in `install/charts/peerpods`.
    
    -   `values.yaml`
        -   `cloudProvider`: set to `kubevirt`
        -   `image`:
            -   `repository`: set the repository of the CAA image
            -   `tag`: set the tag of the CAA image

    Create the `install/charts/peerpods/providers/kubevirt` directory and place the following files in it.  
    Place a file that contains credentials for accessing the [Kubevirt cluster](#kubevirt-cluster).  
    Use the file obtained in the KubeVirt Cluster section. Name this file `kubeconfig`.

    Create a YAML file for creating a Pod VM in KubeVirt.  
    Name this file `podvm.yaml`.  
    An example is shown below.

    ```yaml
    apiVersion: kubevirt.io/v1
    kind: VirtualMachine
    metadata:
      creationTimestamp: null
      name: <vmname>
      namespace: default
    spec:
      runStrategy: Always
      template:
        metadata:
          creationTimestamp: null
        spec:
          domain:
            devices:
              disks:
              - disk:
                  bus: virtio
                cache: writethrough
                name: containerdisk
              rng: {}
            resources:
              requests:
                memory: 4Gi
          terminationGracePeriodSeconds: 180
          volumes:
          - containerDisk:
              image: <image name>
              path: <export qcow2 path>
            name: containerdisk
    status: {}
    ```

    > **Note**: The Pod VM name specified in the YAML file will not be used; the Provider generates the name internally.  
      Therefore, you may specify any name you like in the YAML file.

3.  Deploy CAA using Helm
    Deploy CAA to the Kubernetes cluster using the provided Helm chart.  
    The `Makefile` defines a deployment target that uses the `helm install` command. You can deploy CAA with the following command.

    ```bash
    export CLOUD_PROVIDER=kubevirt
    make deploy
    ```
    
4.  Clean up CAA
    After confirming CAA is working correctly, you can clean it up.

    ```bash
    export CLOUD_PROVIDER=kubevirt
    make delete
    ```
 
## Run a Sample Pod Using Peer Pod VM

Create and run a sample Pod that uses Peer Pod VM on a Kubernetes cluster.

1.  Label Kubernetes worker nodes  
    Label Kubernetes worker nodes to indicate they can run Pods with the Kata runtime.  
    This is required to schedule the sample Pod onto worker nodes.

    ```bash
    kubectl label node <node-name> katacontainers.io/kata-runtime=true
    ```

2.  Verify that RuntimeClass exists
    After deploying CAA, verify that the RuntimeClass has been created.

    ```bash
    kubectl get runtimeclass
    NAME          HANDLER       AGE
    kata-remote   kata-remote   <time>
    ```

3.  Create a YAML file for the sample Pod
    Create a YAML file for the sample Pod (for example, `busybox.yaml`) with the following content.

    ```yaml
    apiVersion: v1
    kind: Pod
    metadata:
      labels:
        run: busybox
      name: busybox
    spec:
      containers:
      - image: quay.io/prometheus/busybox
        name: busybox
        resources: {}
      dnsPolicy: ClusterFirst
      restartPolicy: Never
      runtimeClassName: kata-remote
    ```

4.  Run the sample Pod using the YAML file
    Apply the YAML file to create the sample Pod on the Kubernetes cluster.

    ```bash
    kubectl apply -f busybox.yaml
    ```

5.  Verify the sample Pod is running
    Check the status of the sample Pod and confirm that it is running correctly.

    ```bash
    kubectl get pods busybox -n default
    ```

6.  Stop the sample Pod
    After confirming the sample Pod runs correctly, stop it.

    ```bash
    kubectl delete -f busybox.yaml
    ```
    
## End-to-End (e2e) Test

You can also run the e2e test to verify that CAA is working correctly on your KubeVirt cluster.   
Please refer to the [e2e test documentation](../test/e2e/README.md) for instructions on how to run the e2e test.  
This section assumes your Kubernetes cluster is already set up and that you have permission to deploy applications to that cluster.  

And it is assumed that you have already set up a Kubernetes cluster and have the necessary permissions to deploy applications on it.

1.  Install and configure required tools and packages  
    Building and pushing a CAA image requires several tools and packages. Install and configure the following:

    -   docker-engine or podman
    -   make
    -   yq
    -   golang

2.  Label Kubernetes worker nodes  
    Label Kubernetes worker nodes to indicate they can run Pods with the Kata runtime.    
    This is required to schedule the sample Pod onto worker nodes.

    ```bash
    kubectl label node <node-name> katacontainers.io/kata-runtime=true
    ```

3.  Create a Properties File for the e2e Test
    Create the `kubevirt.properties` file at any path you like, and add the necessary information for the e2e test.  
    Please create the kubeconfig and podvm.yaml files described in the [Deploy CAA](#deploy-caa) section and enter the paths.

    ```properties
    CAA_IMAGE=<image-url>:<tag>

    path_to_kubeconfig="path/to/kubeconfig"
    path_to_vmconfig="path/to/podvm.yaml"
    ```

4.  Run the e2e Test  
    After creating the properties file, you can run the e2e test.

    ```bash
    export CLOUD_PROVIDER=kubevirt
    export TEST_PROVISION_FILE=path/to/kubevirt.properties
    make test-e2e
    ```
