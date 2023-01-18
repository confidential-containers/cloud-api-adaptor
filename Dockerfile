FROM --platform=$BUILDPLATFORM golang:1.18 AS builder
ARG TARGETARCH
ARG CLOUD_PROVIDER
ENV CLOUD_PROVIDER=${CLOUD_PROVIDER}
COPY . cloud-api-adaptor
RUN git clone -b CCv0 https://github.com/kata-containers/kata-containers
WORKDIR cloud-api-adaptor
RUN if [ "$CLOUD_PROVIDER" = "libvirt" ] ; then apt-get update -y && apt-get install -y libvirt-dev; fi
RUN ARCH=$TARGETARCH make

FROM --platform=$TARGETPLATFORM fedora:36
ARG CLOUD_PROVIDER
ENV CLOUD_PROVIDER=${CLOUD_PROVIDER}
RUN if [ "$CLOUD_PROVIDER" = "libvirt" ] ; then dnf install -y libvirt-libs genisoimage /usr/bin/ssh && dnf clean all; fi
COPY --from=builder /go/cloud-api-adaptor/cloud-api-adaptor /usr/local/bin/cloud-api-adaptor-$CLOUD_PROVIDER
COPY --from=builder /go/cloud-api-adaptor/entrypoint.sh /usr/local/bin/entrypoint.sh
CMD ["entrypoint.sh"]
