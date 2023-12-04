# :memo: Adding support for a new provider

### Step 1: Initialize and register the cloud provider manager

The provider-specific cloud manager should be placed under `pkg/adaptor/cloud/<provider>/`.

:information_source:[Example code](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/pkg/adaptor/cloud/aws)

### Step 2: Add provider specific code

Under `pkg/adaptor/cloud/<provider>`, start by adding a new file called `types.go`. This file defines a configuration struct that contains the required parameters for a cloud provider.

:information_source:[Example code](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/pkg/adaptor/cloud/aws/types.go)

#### Step 2.1: Implement the Cloud interface

Create a provider-specific manager file called `manager.go`, which implements the following methods for parsing command-line flags, loading environment variables, and creating a new provider.

- ParseCmd
- LoadEnv
- NewProvider

Create an `init` function to add your manager to the cloud provider table.

```go
func init() {
	cloud.AddCloud("aws", &Manager{})
}
```

:information_source:[Example code](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/pkg/adaptor/cloud/aws/manager.go)

#### Step 2.2: Implement the Provider interface

The Provider interface defines a set of methods that need to be implemented by the cloud provider for managing virtual instances. Add the required methods:

 - CreateInstance
 - DeleteInstance
 - Teardown

:information_source:[Example code](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/pkg/adaptor/cloud/aws/provider.go#L76-L175)

Also, consider adding additional files to modularize the code. You can refer to existing providers such as `aws`, `azure`, `ibmcloud`, and `libvirt` for guidance. Adding unit tests wherever necessary is good practice.

#### Step 2.3: Include Provider package from main

To include your provider you need reference it from the main package. Go build tags are used to selectively include different providers.

:information_source:[Example code](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/cmd/cloud-api-adaptor/aws.go)

```go
//go:build aws
```
Note the comment at the top of the file, when building ensure `-tags=` is set to include your new provider. See the [Makefile](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/Makefile#L26) for more context and usage.

#### Step 3: Add documentation on how to build a Pod VM image

For using the provider, a pod VM image needs to be created in order to create the peer pod instances. Add the instructions for building the peer pod VM image at the root directory similar to the other providers.

#### Step 4: Add E2E tests for the new provider

For more information, please refer to the section on [adding support for a new cloud provider](../test/e2e/README.md#adding-support-for-a-new-cloud-provider) in the E2E testing documentation.
