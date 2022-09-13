# :memo: Adding support for a new provider

### Step 1: Add provider specific options 

Provider specific options goes under `cmd/cloud-api-adaptor`
- [Options](https://github.com/confidential-containers/cloud-api-adaptor/blob/staging/cmd/cloud-api-adaptor/main.go#L48)
- [Parsing](https://github.com/confidential-containers/cloud-api-adaptor/blob/staging/cmd/parse.go#L21)
- [Calling the specific provider](https://github.com/confidential-containers/cloud-api-adaptor/blob/staging/cmd/cloud-api-adaptor/main.go#L103)


### Step 2: Add provider specific code 

- The code goes under `pkg/adaptor/hypervisor/<provider>`
- Use BUILD TAGs to build the provider  (eg // +build libvirt). 

:information_source: Note that there will be separate binaries for each provider.

#### Step 2.1: Add provider entry point

Add provider entry point to the registry. The `registry.newServer` method is the entry point for the provider code.

:information_source: [Example code](https://github.com/confidential-containers/cloud-api-adaptor/blob/staging/pkg/adaptor/hypervisor/registry/libvirt.go)

#### Step 2.2: Implement NewServer method

The `NewServer` method creates the service which is responsible for VM lifecycle operations.

Each provider implements `NewServer` method. 

By convention this should be in the file `<provider>/server.go`

:information_source:[Example code](https://github.com/confidential-containers/cloud-api-adaptor/blob/staging/pkg/adaptor/hypervisor/libvirt/server.go#L36)

#### Step 2.3: Implement NewService method

Each provider implements `newService` method. 

By convention this should be in the file `<provider>/service.go`

:information_source:[Example code](https://github.com/confidential-containers/cloud-api-adaptor/blob/staging/pkg/adaptor/hypervisor/libvirt/service.go#L44)

#### Step 2.4: Implement Kata specific methods
    
Add required methods 
 - CreateVM
 - StartVM
 - StopVM
 - Version

These methods are required by Kata and a Kata hypervisor needs to implement these methods.

#### Step 2.5: Implement Kata specific methods

Add additional files to modularize the code.

See existing providers - `aws|azure|ibmcloud|libvirt`

#### Step 3: Update Continuous Integration (CI) workflows

Each provider should be built and tested on CI.

Update the `provider` list under the `matrix` property in [`.github/workflows/build.yaml`](../.github/workflows/build.yaml).
