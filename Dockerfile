FROM fedora:36
ARG ARCH
ARG CLOUD_PROVIDER
ENV CLOUD_PROVIDER=${CLOUD_PROVIDER}
RUN if [ "$CLOUD_PROVIDER" = "libvirt" ] ; then dnf install -y libvirt-libs genisoimage /usr/bin/ssh && dnf clean all; fi
COPY cloud-api-adaptor-${ARCH} /usr/local/bin/cloud-api-adaptor-$CLOUD_PROVIDER
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
CMD ["entrypoint.sh"]
