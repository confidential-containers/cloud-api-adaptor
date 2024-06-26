# Copyright Confidential Containers Contributors
#
# SPDX-License-Identifier: Apache-2.0
#
# Builds pod vm image inside container
#
# syntax=docker/dockerfile:1.3
ARG BUILDER_IMG="quay.io/confidential-containers/podvm-builder-ubuntu"
ARG BINARIES_IMG="quay.io/confidential-containers/podvm-binaries-ubuntu-amd64"

FROM ${BINARIES_IMG} AS podvm_binaries
FROM ${BUILDER_IMG} AS podvm_builder

ARG CLOUD_PROVIDER=azure
ARG PODVM_DISTRO=ubuntu
# If not provided, uses system architecture
ARG ARCH=x86_64
ARG CAA_SRC=""
ARG CAA_SRC_REF=""

ENV CLOUD_PROVIDER=${CLOUD_PROVIDER}
ENV PODVM_DISTRO=${PODVM_DISTRO}

RUN if [ -n "${CAA_SRC}" ]; then \
      rm -rf cloud-api-adaptor && \
      git clone ${CAA_SRC} cloud-api-adaptor;\
    fi && \
    if [ -n "${CAA_SRC_REF}" ]; then \
      cd cloud-api-adaptor && \
      git fetch origin ${CAA_SRC_REF} && \
      git checkout FETCH_HEAD -b ${CAA_SRC_REF} ;\
    fi

# Copy the binaries to podvm/files folder
COPY --from=podvm_binaries /podvm-binaries.tar.gz /src/cloud-api-adaptor/podvm/files
RUN tar xvf /src/cloud-api-adaptor/podvm/files/podvm-binaries.tar.gz -C /src/cloud-api-adaptor/podvm/files

# Copy the pause_bundle to podvm/files folder
COPY --from=podvm_binaries /pause-bundle.tar.gz /src/cloud-api-adaptor/podvm/files
RUN tar xvf /src/cloud-api-adaptor/podvm/files/pause-bundle.tar.gz -C /src/cloud-api-adaptor/podvm/files

ENV VM_SIZE=Standard_D2as_v5
ENV IMAGE_NAME=peer-pod-vmimage
ENV SSH_USERNAME=peerpod
ENV PUBLISHER=Canonical
ENV OFFER=0001-com-ubuntu-confidential-vm-jammy
ENV SKU=22_04-lts-cvm

RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash

WORKDIR /src/cloud-api-adaptor/azure/image

CMD export PKR_VAR_subscription_id=${AZURE_SUBSCRIPTION_ID} && \
    export PKR_VAR_resource_group=${AZURE_RESOURCE_GROUP} && \
    export PKR_VAR_use_azure_cli_auth=true \
    export PKR_VAR_az_image_name=${IMAGE_NAME} && \
    export PKR_VAR_az_gallery_name=${GALLERY_NAME} && \
    export PKR_VAR_az_gallery_image_name=${GALLERY_IMAGE_DEF_NAME} && \
    export PKR_VAR_vm_size=${VM_SIZE} && \
    export PKR_VAR_ssh_username=${SSH_USERNAME} && \
    export PKR_VAR_publisher=${PUBLISHER} && \
    export PKR_VAR_offer=${OFFER} && \
    export PKR_VAR_sku=${SKU} && \
    export PKR_VAR_plan_name=${PLAN_NAME} && \
    export PKR_VAR_plan_product=${PLAN_PRODUCT} && \
    export PKR_VAR_plan_publisher=${PLAN_PUBLISHER} && \
    BINARIES= PAUSE_BUNDLE= CLOUD_PROVIDER=azure PODVM_DISTRO=$PODVM_DISTRO make image
