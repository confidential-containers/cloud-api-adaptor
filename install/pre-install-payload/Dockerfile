ARG IMAGE
FROM ${IMAGE:-docker.io/library/centos}:7

ARG ARCH
ARG SYSTEMD_ARTIFACTS=./config/remote-hyp.service
ARG CAA_ARTIFACTS=./scripts
ARG DESTINATION=/opt/confidential-containers-pre-install-artifacts

COPY ${SYSTEMD_ARTIFACTS} ${DESTINATION}/etc/systemd/system/
COPY ${CAA_ARTIFACTS}/* ${DESTINATION}/scripts/

RUN \
echo "[kubernetes]" >> /etc/yum.repos.d/kubernetes.repo && \
echo "name=Kubernetes" >> /etc/yum.repos.d/kubernetes.repo && \
echo "baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-$(uname -m)" >> /etc/yum.repos.d/kubernetes.repo && \
echo "gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg" >> /etc/yum.repos.d/kubernetes.repo && \
yum -y install kubectl && \
yum clean all
