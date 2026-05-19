This document describes how to build and deploy Confidential Containers (CoCo) Cloud-API-Adaptor (CAA) for OpenStack cluster and run a sample pod using a Peer Pod VM.

> **Note**: The CAA, which specifies an OpenStack provider, is intended for general OpenStack cluster. OpenStack is a highly customizable cloud platform, and customization may be required depending on the specific OpenStack distribution.

## Pod VM Image

The Pod VM Image is a custom image used to create the Peer Pod VM. It contains the necessary components and configurations for the Peer Pod VM to function properly.

1.  Use an Existing Pod VM Image, or Build Your Own Pod VM Image
    Pod VM images can be pulled from public registries such as `quay.io`. For example, you can use the image `quay.io/confidential-containers/podvm-generic-ubuntu-<ARCH>`, where `<ARCH>` is the architecture of your OpenStack cluster (e.g., `amd64`, `arm64`, etc.).

    You can also build your own Pod VM image by following the instructions in the [Pod VM README](../podvm/README.md). After building the image, you can push it to any Docker registry of your choice. The registry URL, image name, and tag will be needed in later steps, so please make a note of them.

2.  Export podvm-*.qcow2 from the Pod VM Image  
    Export the podvm-*.qcow2 file from the Pod VM image, which will be used to create the Peer Pod VM on the OpenStack cluster.  
    You can use the provided script to export the podvm-*.qcow2 file from the Pod VM image. Run the following command, replacing `<image:tag>`, `<output directory>`, and `<output file name>` with the appropriate values:

    ```bash
    podvm/hack/download-image.sh <image:tag> <output directory> -o <output file name>
    ```

## OpenStack Cluster

It is assumed that you have already set up an OpenStack cluster and have the necessary permissions to create resources on it.

> **Note**: This document does not cover how to set up an OpenStack cluster. Please refer to the official OpenStack documentation for instructions on how to set up an OpenStack cluster.

1.  Get Credentials to Access an OpenStack Cluster  
    Credentials are needed for CAA to authenticate and interact with your OpenStack cluster. Obtain the necessary credentials to access your OpenStack cluster, such as:

    -   Identity endpoint URL
    -   Username
    -   Password
    -   Tenant (Project) name
    -   Domain name
    -   Region name

2.  Register the Pod VM Image with Your OpenStack Cluster  
    Register the Pod VM image (podvm-*.qcow2) with your OpenStack cluster as a custom image. If you are using the OpenStack command-line client, you can run the following commands:

    ```bash
    openstack image create <image-name> --file <image-file> --disk-format qcow2 --container-format bare --public
    ```

3.  Set Up Other Necessary Resources on the OpenStack Cluster  
    Set up other necessary resources on your OpenStack cluster:

    -   A security group for the Pod VM to allow necessary traffic between the Pod VM and the Kubernetes worker node.
    -   A flavor for the Pod VM.
    -   Networks for the Pod VM to connect.
    -   (Optional) A network for a floating IP, if needed.

## CAA Image

The CAA image is a container image that contains the CAA components. You can either use a pre-built CAA image or build your own CAA image from the source code. If you want to use your own built CAA image, follow the instructions below to build and push the CAA image to a Docker registry of your choice.

### Create CAA Image

This step is only necessary if you want to build your own CAA image. If you want to use a pre-built CAA image, you can skip this step.

1.  Install and Set Up Necessary Tools and Packages  
    Some tools and packages are required to build and push the CAA image. Install and set up the necessary tools and packages as follows:

    -   docker-engine or podman
    -   make
    -   yq
    -   golang

2.  Clone the CAA Source Tree  
    Clone the CAA source tree from the official repository.

    ```bash
    cd path/to/your/working/directory
    git clone <repository-url> -b <branch-name>
    cd cloud-api-adaptor/src/cloud-api-adaptor
    ```

3.  Build and Push the CAA Image  
    A `make` target for building and pushing the CAA image is defined in the `Makefile`. The `make` command will build the CAA image and push it to the specified Docker registry.  
    Default image tags are set to "latest" and "dev-<VERSION>".  
    "COMMIT" must be specified in three-digit number format (vN.N.N). If "COMMIT" is not set, the version tag value assigned to the local repository is used.

    ```bash
    export CLOUD_PROVIDER=openstack
    export registry=<registry-url>
    export COMMIT=<version tag>
    export VERSION=<image tag>
    make image
    ```

### Deploy CAA

Deploy CAA to your kubernetes cluster using the provided Helm chart.
It is assumed that you have already set up a Kubernetes cluster and have the necessary permissions to deploy applications on it.

1.  Install and Set Up Necessary Tools and Packages  
    Some tools and packages are required to deploy the CAA image. Install and set up the necessary tools and packages as follows:

    -   make
    -   yq
    -   golang
    -   helm

2.  Set Up the Helm Chart for the OpenStack Provider  
    Edit the CAA configuration files to set up the authentication information for your OpenStack cluster and the conditions for starting the Peer Pod VM. The configuration files for the OpenStack provider are located in the `install/charts/peerpods` directory.

    -   `values.yaml`
        -   `cloudProvider`:  
            Set to "openstack".
        -   `image`:  
            -   `repository`:  
                Set to the repository of the CAA image.
            -   `tag`:  
                Set to the tag of the CAA image.

    -   `providers/openstack.yaml`
        -   `OPENSTACK_IMAGE_ID`:  
            Set to the image ID of the Pod VM image you registered with your OpenStack cluster.
        -   `OPENSTACK_FLAVOR_ID`:  
            Set to the flavor you created for the Pod VM.
        -   `OPENSTACK_SECURITY_GROUP`:  
            Set to the security group you created for the Pod VM.
        -   `OPENSTACK_NETWORK_ID`:  
            Set to the network you created for the Pod VM.
        -   (Optional) `OPENSTACK_FLOATING_IP_NETWORK_ID`:  
            Set to the network you created for a floating IP, if needed.

    -   `providers/openstack-secrets.yaml`  
        (Copy from `providers/openstack-secrets.yaml.template`)
        -   `OPENSTACK_IDENTITY_ENDPOINT`:  
            Set to the identity endpoint URL of your OpenStack cluster.
        -   `OPENSTACK_USERNAME`:  
            Set to the username for authentication.
        -   `OPENSTACK_PASSWORD`:  
            Set to the password for authentication.
        -   `OPENSTACK_TENANT_NAME`:  
            Set to the tenant (project) name for authentication.
        -   `OPENSTACK_DOMAIN_NAME`:  
            Set to the domain name for authentication.
        -   `OPENSTACK_REGION_NAME`:  
            Set to the region name of your OpenStack cluster.

3.  Deploy CAA Using Helm  
    Deploy CAA to your kubernetes cluster using the provided Helm chart. A deploy target using the `helm install` command is defined in the `Makefile`; you can run the following command to deploy CAA:

    ```bash
    export CLOUD_PROVIDER=openstack
    make deploy
    ```

4.  Clean Up CAA  
    After verifying that CAA is working correctly, you can clean up the CAA.

    ```bash
    export CLOUD_PROVIDER=openstack
    make delete
    ```

## Run a Sample Pod Using a Peer Pod VM

Create and run a sample pod that uses the Peer Pod VM on the OpenStack cluster.

1.  Label a Kubernetes Worker Node  
    Label a Kubernetes worker node to indicate that it can run pods with the Kata runtime. This is necessary for scheduling the sample pod to run on the worker node.

    ```bash
    kubectl label node <node-name> katacontainers.io/kata-runtime=true
    ```

2.  Ensure runtimeclass is present 
    Verify that the runtimeclass is created after deploying CAA.  

    ```bash
    kubectl get runtimeclass
    NAME          HANDLER       AGE
    kata-remote   kata-remote   <time>
    ```

3.  Create a YAML File for the Sample Pod  
    Create a YAML file for the sample pod, for example, `busybox.yaml`, with the following content:

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

4.  Run the Sample Pod Using the YAML File  
    Apply the YAML file to create the sample pod on the OpenStack cluster.

    ```bash
    kubectl apply -f busybox.yaml
    ```
5.  Ensure that the Sample Pod is Running  
    Check the status of the sample pod to ensure that it is running correctly.  

    ```bash
    kubectl get pods busybox -n default
    ```

6.  Stop the Sample Pod  
    After verifying that the sample pod is running correctly, you can stop the sample pod.

    ```bash
    kubectl delete -f busybox.yaml
    ```

## End-to-End (e2e) Test

You can also run the e2e test to verify that CAA is working correctly on your OpenStack cluster. Please refer to the [e2e test documentation](../test/e2e/README.md) for instructions on how to run the e2e test.

The OpenStack provisioner does not support the creation or setup of an OpenStack Cluster. You must assume that the cluster is ready with the necessary setup and create the `openstack.properties` file with the necessary information to run the e2e test.

And it is assumed that you have already set up a Kubernetes cluster and have the necessary permissions to deploy applications on it.

1.  Install and Set Up Necessary Tools and Packages  
    Some tools and packages are required to run the e2e test. Install and set up the necessary tools and packages as follows:

    -   make
    -   yq
    -   golang
    -   helm

2.  Label a Kubernetes Worker Node  
    Label a Kubernetes worker node to indicate that it can run pods with the Kata runtime. This is necessary for scheduling the pod to run on the worker node.

    ```bash
    kubectl label node <node-name> katacontainers.io/kata-runtime=true
    ```

3.  Create a Properties File for the e2e Test  
    Create the `openstack.properties` file at any path you like, and add the necessary information for the e2e test.

    ```properties
    CAA_IMAGE=<image-url>:<tag>
    OPENSTACK_IMAGE_ID=<image-id>
    OPENSTACK_FLAVOR_ID=<flavor-id>
    OPENSTACK_SECURITY_GROUP=<security-group>
    OPENSTACK_NETWORK_ID=<network-id>
    OPENSTACK_FLOATING_IP_NETWORK_ID=<floating-network-id>
    OPENSTACK_CREDENTIALS=<path-to-secrets-file>
    ```

4.  Create a Secrets File for the e2e Test  
    Create a secrets file that contains the credentials for your OpenStack cluster.

    ```properties
    OPENSTACK_IDENTITY_ENDPOINT=<identity-endpoint-url>
    OPENSTACK_USERNAME=<username>
    OPENSTACK_PASSWORD=<password>
    OPENSTACK_TENANT_NAME=<tenant-name>
    OPENSTACK_DOMAIN_NAME=<domain-name>
    OPENSTACK_REGION_NAME=<region-name>
    ```

5.  Run the e2e Test  
    After creating the properties file and secrets file, you can run the e2e test.

    ```bash
    export CLOUD_PROVIDER=openstack
    export TEST_PROVISION_FILE=path/to/openstack.properties
    make test-e2e
    ```

