# Copyright Confidential Containers Contributors
#
# SPDX-License-Identifier: Apache-2.0
#
# Builds pod vm image inside container
#
# syntax=docker/dockerfile:1.3
ARG RELEASE_TAG="v0.9.0"
ARG BUILDER_IMG="quay.io/confidential-containers/podvm-builder-ubuntu:${RELEASE_TAG}"
ARG BINARIES_IMG="quay.io/confidential-containers/podvm-binaries-ubuntu-amd64:${RELEASE_TAG}"

FROM ${BINARIES_IMG} AS podvm_binaries
FROM ${BUILDER_IMG} AS podvm_builder

ARG CLOUD_PROVIDER=aws
ARG PODVM_DISTRO=ubuntu
# If not provided, uses system architecture
ARG ARCH=x86_64
ARG CAA_SRC="https://github.com/confidential-containers/cloud-api-adaptor"
ARG CAA_SRC_REF=${RELEASE_TAG}

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
COPY --from=podvm_binaries /podvm-binaries.tar.gz /src/cloud-api-adaptor/podvm/files/
RUN tar xvf /src/cloud-api-adaptor/podvm/files/podvm-binaries.tar.gz -C /src/cloud-api-adaptor/podvm/files

# Copy the pause_bundle to podvm/files folder
COPY --from=podvm_binaries /pause-bundle.tar.gz /src/cloud-api-adaptor/podvm/files/
RUN tar xvf /src/cloud-api-adaptor/podvm/files/pause-bundle.tar.gz -C /src/cloud-api-adaptor/podvm/files

ARG AWS_REGION
ARG VPC_ID
ARG SUBNET_ID
ARG INSTANCE_TYPE=t3.small
ARG IMAGE_NAME=peer-pod-ami
ARG VOLUME_SIZE=30

ENV AWS_REGION=${AWS_REGION}
ENV VPC_ID=${VPC_ID}
ENV SUBNET_ID=${SUBNET_ID}
ENV INSTANCE_TYPE=${INSTANCE_TYPE}
ENV IMAGE_NAME=${IMAGE_NAME}
ENV VOLUME_SIZE=${VOLUME_SIZE}

# Install AWS cli which will be used for default-vpc creation if required
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
     unzip awscliv2.zip && \
     ./aws/install


RUN --mount=type=secret,id=AWS_ACCESS_KEY_ID \
    --mount=type=secret,id=AWS_SECRET_ACCESS_KEY \
    --mount=type=secret,id=AWS_SESSION_TOKEN \
    export AWS_ACCESS_KEY_ID=$(cat /run/secrets/AWS_ACCESS_KEY_ID) && \
    export AWS_SECRET_ACCESS_KEY=$(cat /run/secrets/AWS_SECRET_ACCESS_KEY) && \
    export AWS_SESSION_TOKEN=$(cat /run/secrets/AWS_SESSION_TOKEN 2>/dev/null || echo "") && \
    cd cloud-api-adaptor/src/cloud-api-adaptor/aws/image && \
    BINARIES= PAUSE_BUNDLE= CLOUD_PROVIDER=aws PODVM_DISTRO=$PODVM_DISTRO make image
