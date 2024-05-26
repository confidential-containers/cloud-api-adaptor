ARG IMAGE=kindest/node
ARG VERSION=1.27
ARG MINOR=11

FROM ${IMAGE}:v${VERSION}.${MINOR}

RUN apt-get update && apt-get install -y sudo  && apt-get clean all


RUN systemctl disable kubelet
RUN systemctl disable containerd

COPY ./resources/binaries-tree/etc/ /etc/
COPY ./resources/binaries-tree/usr/ /usr/
COPY ./resources/binaries-tree/pause_bundle/ /pause_bundle/

RUN curl -LO https://raw.githubusercontent.com/confidential-containers/cloud-api-adaptor/main/src/cloud-api-adaptor/podvm/qcow2/misc-settings.sh

RUN PODVM_DISTRO=ubuntu CLOUD_PROVIDER=generic DISABLE_CLOUD_CONFIG=true bash ./misc-settings.sh
