ARG IMAGE=kindest/node
ARG VERSION=1.27
ARG MINOR=11

FROM ${IMAGE}:v${VERSION}.${MINOR}

RUN apt-get update && apt-get install -y wget sudo


ADD ./resources/binaries-tree/etc/ etc/
ADD ./resources/binaries-tree/usr/ usr/
ADD ./resources/binaries-tree/pause_bundle /


RUN wget https://raw.githubusercontent.com/confidential-containers/cloud-api-adaptor/main/podvm/qcow2/misc-settings.sh

RUN PODVM_DISTRO=ubuntu CLOUD_PROVIDER=generic DISABLE_CLOUD_CONFIG=true bash ./misc-settings.sh

RUN systemctl disable kubelet
RUN systemctl disable containerd

