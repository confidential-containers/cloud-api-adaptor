FROM --platform=$BUILDPLATFORM golang:1.19 AS builder
ARG TARGETARCH
ARG RELEASE_BUILD
ENV RELEASE_BUILD=${RELEASE_BUILD}
COPY . cloud-api-adaptor
WORKDIR cloud-api-adaptor
# Install additional packages required to build libvirt provider
# Need to use the [] syntax as default shell is /bin/sh
RUN if [ "$RELEASE_BUILD" != "true" ] ; then apt-get update -y && apt-get install -y libvirt-dev; fi
RUN ARCH=$TARGETARCH make

FROM --platform=$TARGETPLATFORM fedora:36
ARG RELEASE_BUILD
ENV RELEASE_BUILD=${RELEASE_BUILD}
# Install additional packages required when using libvirt provider
# Need to use the [] syntax as default shell is /bin/sh
RUN if [ "$RELEASE_BUILD" != "true" ] ; then dnf install -y libvirt-libs genisoimage /usr/bin/ssh && dnf clean all; fi
COPY --from=builder /go/cloud-api-adaptor/cloud-api-adaptor /usr/local/bin/cloud-api-adaptor
COPY --from=builder /go/cloud-api-adaptor/entrypoint.sh /usr/local/bin/entrypoint.sh
CMD ["entrypoint.sh"]
