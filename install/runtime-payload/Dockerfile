ARG IMAGE
FROM ${IMAGE:-quay.io/confidential-containers/runtime-payload-ci:kata-containers-latest}

ARG ARCH
ARG BINARIES=./bin
ARG SCRIPTS=./scripts
ARG CONFIG=./config

#Copy containerd-shim to /opt/confidential-containers/bin
ADD ${BINARIES}/containerd-shim-kata-v2-${ARCH} /opt/kata-artifacts/opt/confidential-containers/bin/containerd-shim-kata-v2

#Copy configuration file
ADD ${CONFIG}/configuration-remote.toml /opt/kata-artifacts/opt/confidential-containers/share/defaults/kata-containers/configuration.toml

#Copy kata-deploy.sh
ADD ${SCRIPTS}/kata-deploy.sh /opt/kata-artifacts/scripts/
