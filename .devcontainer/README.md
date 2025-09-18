# Minimal edit container (Alpine)
This container are the bare minimum and can be used if no depedencies are needed, like Go etc.

# General development container

This container has been setup so all tasks can be done on general level and includes:
* Go
* YQ so that mkosi builds can run (building peerpodvm images)
* qemu-utils needed for raw -> qcow2 conversion after mkosi build
* Uplosi so built images can be uploaded to the different cloudproviders


> **Note:** Note: Cloud provider-specific tools (e.g., Azure CLI, AWS CLI, Google Cloud SDK) are not included by default to keep the development environment lightweight. Developers can install these tools as needed based on their specific use cases.

> **Note:** For guidance on using `mkosi` to build and upload images to an Azure Image Gallery, see the [uplosi_azure_notes.md](./development/uplosi_azure_notes.md).
