ARG IMAGE
FROM ${IMAGE:-quay.io/confidential-containers/runtime-payload:kata-containers-b11b6e3756a15e602a5bff1d4d47da8ecf23c593}

ARG ARCH
ARG BINARIES=./bin
ARG SCRIPTS=./scripts
ARG CONFIG=./config

#Copy containerd-shim to /opt/confidential-containers/bin
ADD ${BINARIES}/containerd-shim-kata-v2 /opt/kata-artifacts/opt/confidential-containers/bin/

#Copy configuration file
ADD ${CONFIG}/configuration-remote.toml /opt/kata-artifacts/opt/confidential-containers/share/defaults/kata-containers

#Adjust the default config link
RUN cd /opt/kata-artifacts/opt/confidential-containers/share/defaults/kata-containers && \
       rm -f configuration.toml && \
       ln -sf configuration-remote.toml configuration.toml

#Copy kata-deploy.sh
ADD ${SCRIPTS}/kata-deploy.sh /opt/kata-artifacts/scripts/
