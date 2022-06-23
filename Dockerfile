FROM golang:1.17.3

ADD . cloud-api-adaptor
RUN git clone -b CCv0-peerpod https://github.com/yoheiueda/kata-containers
RUN apt-get update && apt-get install -y libvirt-dev libvirt0 genisoimage # other requirements?
WORKDIR cloud-api-adaptor
# generic build
ENV CLOUD_PROVIDER=aws
RUN make
RUN install cloud-api-adaptor /usr/local/bin/cloud-api-adaptor
RUN make clean
# libvirt build
ENV CLOUD_PROVIDER=libvirt
RUN make
RUN install cloud-api-adaptor /usr/local/bin/cloud-api-adaptor-libvirt

ENV CLOUD_PROVIDER=none
ADD entrypoint.sh /usr/local/bin/
CMD entrypoint.sh
