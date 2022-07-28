FROM golang:1.18 AS builder
ARG CLOUD_PROVIDER
ENV CLOUD_PROVIDER=${CLOUD_PROVIDER}
COPY . cloud-api-adaptor
RUN git clone -b CCv0-peerpod https://github.com/yoheiueda/kata-containers
WORKDIR cloud-api-adaptor
RUN make clean
RUN if [ "$CLOUD_PROVIDER" = "libvirt" ] ; then apt-get update -y && apt-get install -y libvirt-dev libvirt0 genisoimage && apt-get clean; fi
RUN make
RUN install cloud-api-adaptor /usr/local/bin/cloud-api-adaptor-$CLOUD_PROVIDER
RUN install entrypoint.sh /usr/local/bin/
RUN make clean
CMD entrypoint.sh
