ARG IMAGE
FROM ${IMAGE:-quay.io/confidential-containers/runtime-payload:kata-containers-e8902bb373727d282f1ba7612056f671b8096ee4}

ARG ARCH
ARG BINARIES=./bin
ARG SCRIPTS=./scripts
ARG CONFIG=./config

#Copy containerd-shim to /opt/confidential-containers/bin
ADD ${BINARIES}/containerd-shim-kata-v2 /opt/kata-artifacts/opt/confidential-containers/bin/
ADD ${BINARIES}/kata-runtime /opt/kata-artifacts/opt/confidential-containers/bin/
ADD ${BINARIES}/kata-monitor /opt/kata-artifacts/opt/confidential-containers/bin/
#Copy cloud-api-adaptor /opt/confidential-containers/bin
ADD ${BINARIES}/cloud-api-adaptor-aws /opt/kata-artifacts/opt/confidential-containers/bin/
ADD ${BINARIES}/cloud-api-adaptor-ibmcloud /opt/kata-artifacts/opt/confidential-containers/bin/
ADD ${BINARIES}/cloud-api-adaptor-libvirt /opt/kata-artifacts/opt/confidential-containers/bin/

#Copy scripts
ADD ${SCRIPTS}/run-aws.sh /opt/kata-artifacts/opt/confidential-containers/bin/
ADD ${SCRIPTS}/run-ibmcloud.sh /opt/kata-artifacts/opt/confidential-containers/bin/
ADD ${SCRIPTS}/run-libvirt.sh /opt/kata-artifacts/opt/confidential-containers/bin/


#Copy configuration file
ADD ${CONFIG}/configuration-remote.toml /opt/kata-artifacts/opt/confidential-containers/share/defaults/kata-containers

#Adjust the default config link
RUN cd /opt/kata-artifacts/opt/confidential-containers/share/defaults/kata-containers && \
       rm -f configuration.toml && \
       ln -sf configuration-remote.toml configuration.toml

#Copy kata-deploy.sh
ADD ${SCRIPTS}/kata-deploy.sh /opt/kata-artifacts/scripts/
