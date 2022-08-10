#!/bin/bash

/opt/confidential-containers/bin/cloud-api-adaptor-aws aws \
    -aws-access-key-id ${AWS_ACCESS_KEY_ID} \
    -aws-secret-key ${AWS_SECRET_ACCESS_KEY} \
    -aws-region ${AWS_REGION} \
    -pods-dir /run/peerpod/pods \
    -socket /run/peerpod/hypervisor.sock \
    -aws-lt-name ${PODVM_LAUNCHTEMPLATE_NAME}
