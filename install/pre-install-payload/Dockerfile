FROM --platform=$BUILDPLATFORM quay.io/centos/centos:stream9-minimal as builder

ARG TARGETARCH

RUN \
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/$TARGETARCH/kubectl" && \
install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

FROM --platform=$TARGETPLATFORM quay.io/centos/centos:stream9-minimal

COPY --from=builder /usr/local/bin/kubectl /usr/local/bin/kubectl

ARG SYSTEMD_ARTIFACTS=./config/remote-hyp.service
ARG CAA_ARTIFACTS=./scripts
ARG DESTINATION=/opt/confidential-containers-pre-install-artifacts

COPY ${SYSTEMD_ARTIFACTS} ${DESTINATION}/etc/systemd/system/
COPY ${CAA_ARTIFACTS}/* ${DESTINATION}/scripts/
