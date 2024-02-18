FROM quay.io/bpradipt/osc-payload as payload

FROM antmelekhin/docker-systemd:ubuntu-22.04 as node

ENV PODVM_DISTRO=ubuntu
ARG PODVM_DISTRO=${PODVM_DISTRO}

ENV CLOUD_PROVIDER=generic
ENV CLOUD_PROVIDER=${CLOUD_PROVIDER}

COPY --from=payload /payload/podvm-binaries.tar.gz /tmp/

RUN mkdir -p /tmp/files && tar -xvf /tmp/podvm-binaries.tar.gz -C /tmp/files

COPY --from=payload /src/cloud-api-adaptor/podvm/qcow2/*.sh /

RUN mkdir -p /tmp/files/pause_bundle

RUN /copy-files.sh
RUN /misc-settings.sh


# Running
# docker run --rm -it --privileged --volume=/sys/fs/cgroup:/sys/fs/cgroup --cgroupns=host docker.io/library/test 