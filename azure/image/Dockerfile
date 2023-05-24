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

ARG PODVM_DISTRO=ubuntu
ENV PODVM_DISTRO ${PODVM_DISTRO}

# If not provided, uses system architecture
ARG ARCH=x86_64

# Copy the local source code of cloud api adaptor.
RUN rm -rf /src/cloud-api-adaptor
COPY . /src/cloud-api-adaptor
RUN mkdir -p /src/cloud-api-adaptor/podvm/files

# Copy the binaries to podvm/files folder
COPY --from=podvm_binaries /podvm-binaries.tar.gz /src/cloud-api-adaptor/podvm/files
COPY --from=podvm_binaries /pause-bundle.tar.gz /src/cloud-api-adaptor/podvm/files

# Copy the pause_bundle to podvm/files folder
RUN tar xvf /src/cloud-api-adaptor/podvm/files/podvm-binaries.tar.gz -C /src/cloud-api-adaptor/podvm/files && \
    rm /src/cloud-api-adaptor/podvm/files/podvm-binaries.tar.gz && \
    tar xvf /src/cloud-api-adaptor/podvm/files/pause-bundle.tar.gz -C /src/cloud-api-adaptor/podvm/files && \
    rm /src/cloud-api-adaptor/podvm/files/pause-bundle.tar.gz

RUN cd /src/cloud-api-adaptor/azure/image && \
    BINARIES= PAUSE_BUNDLE= CLOUD_PROVIDER=azure PODVM_DISTRO=$PODVM_DISTRO make image-build-in-docker
