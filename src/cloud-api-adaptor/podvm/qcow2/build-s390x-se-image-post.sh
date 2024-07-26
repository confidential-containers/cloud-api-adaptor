#!/bin/bash

if [ "${SE_BOOT:-false}" != "true" ]; then
    exit 0
elif [ "${ARCH}" != "s390x" ]; then
    echo "Building of SE podvm image is only supported for s390x"
    rm -f "se-${IMAGE_NAME}"
    exit 0
fi
[ ! -e "se-${IMAGE_NAME}" ] && exit 1
rm -f "${OUTPUT_DIRECTORY}/${IMAGE_NAME}"
qemu-img convert -O qcow2 -c se-${IMAGE_NAME} ${OUTPUT_DIRECTORY}/se-${IMAGE_NAME}
rm -f se-${IMAGE_NAME}
echo "SE podvm image for s390x is built: ${OUTPUT_DIRECTORY}/se-${IMAGE_NAME}"
