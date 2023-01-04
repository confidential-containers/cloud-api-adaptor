#!/usr/bin/env bash

[ "$DEBUG" == 'true' ] && set -x

ARGC=$#
if [ $ARGC -ne 3 ]; then
    echo "USAGE: $(basename $0) <Image Path> <S3 Bucket Name> <Region>"
    exit 1
fi
IMAGE_FILE_PATH=$1
BUCKET_NAME=$2
REGION=$3

IMAGE_NAME="$(basename -- ${IMAGE_FILE_PATH})"
AMI_NAME="${IMAGE_NAME%.*}"
FORMAT="${IMAGE_NAME#*.}"

TMPDIR=$(mktemp -d)
IMAGE_IMPORT_JSON_FILE="${TMPDIR}/image-import.json"
AMI_REGISTER_JSON_FILE="${TMPDIR}/register-ami.json"
TRUST_POLICY_JSON_FILE="${TMPDIR}/trust-policy.json"
ROLE_POLICY_JSON_FILE="${TMPDIR}/role-policy.json"
BUCKET_POLICY_JSON_FILE="${TMPDIR}/bucket-policy.json"

pre_checks() {
	[[ -x "$(command -v jq)" ]] || { echo "jq is not installed" 1>&2 ; exit 1; }
	[[ -x "$(command -v aws)" ]] || { echo "aws is not installed" 1>&2 ; exit 1; }
	[[ "$FORMAT" == "raw"  ]] || { echo "image must be \"raw\", convert with: \"qemu-img convert ${IMAGE_NAME} ${AMI_NAME}.raw\"" 1>&2 ; exit 1; }
	aws sts get-caller-identity &>/dev/null || { echo "awc cli missing credentials"; exit 1; }
}

create_bucket() {
	echo "Create s3 Bucket"
	if [[ ${REGION} == us-east-1 ]]; then
		aws s3api create-bucket  --bucket ${BUCKET_NAME} --region ${REGION}
	else
		aws s3api create-bucket  --bucket ${BUCKET_NAME} --region ${REGION} --create-bucket-configuration LocationConstraint=${REGION}
	fi
}

set_bucket_policies() {
	echo "Set bucket policies"
	cat <<EOF > "${TRUST_POLICY_JSON_FILE}"
{
	"Version":"2012-10-17",
	"Statement":[
		{
			"Effect":"Allow",
			"Principal":{ "Service":"vmie.amazonaws.com" },
			"Action": "sts:AssumeRole",
			"Condition":{"StringEquals":{"sts:Externalid":"vmimport"}}
		}
	]
}
EOF

	aws iam create-role --role-name vmimport --assume-role-policy-document "file://${TRUST_POLICY_JSON_FILE}"

	cat <<EOF > "${ROLE_POLICY_JSON_FILE}"
{
	"Version":"2012-10-17",
	"Statement":[
		{
			"Effect":"Allow",
			"Action":["s3:GetBucketLocation","s3:GetObject","s3:ListBucket"],
			"Resource":["arn:aws:s3:::${BUCKET_NAME}","arn:aws:s3:::${BUCKET_NAME}/*"]
		},
		{
			"Effect":"Allow",
			"Action":["ec2:ModifySnapshotAttribute","ec2:CopySnapshot","ec2:RegisterImage","ec2:Describe*"],
			"Resource":"*"
		}
	]
}
EOF

	aws iam put-role-policy --role-name vmimport --policy-name vmimport --policy-document "file://${ROLE_POLICY_JSON_FILE}"

	cat <<EOF > "${BUCKET_POLICY_JSON_FILE}"
{
	"Version": "2012-10-17",
	"Statement": [
	{
		"Sid": "AllowVMIE",
		"Effect": "Allow",
		"Principal": { "Service": "vmie.amazonaws.com" },
		"Action": ["s3:GetBucketLocation", "s3:GetObject", "s3:ListBucket" ],
		"Resource": ["arn:aws:s3:::${BUCKET_NAME}", "arn:aws:s3:::${BUCKET_NAME}/*"]}]
}
EOF

	aws s3api put-bucket-policy --bucket ${BUCKET_NAME} --policy "file://${BUCKET_POLICY_JSON_FILE}"

}

upload_raw_to_bucket() {
	echo "Upload image to bucket"

	aws s3 cp "${IMAGE_FILE_PATH}" "s3://${BUCKET_NAME}" --region "${REGION}" || exit $?
}


import_snapshot_n_wait() {
	echo "Importing image file into snapshot"

	cat <<EOF > "${IMAGE_IMPORT_JSON_FILE}"
{
    "Description": "Peer Pod VM image",
    "Format": "RAW",
    "UserBucket": {
	"S3Bucket": "${BUCKET_NAME}",
	"S3Key": "${IMAGE_NAME}"
    }
}
EOF

	IMPORT_TASK_ID=$(aws ec2 import-snapshot --disk-container "file://${IMAGE_IMPORT_JSON_FILE}" --output json | jq -r '.ImportTaskId')

	IMPORT_STATUS=$(aws ec2 describe-import-snapshot-tasks --import-task-ids $IMPORT_TASK_ID --output json | jq -r '.ImportSnapshotTasks[].SnapshotTaskDetail.Status')
	x=0
	while [ "$IMPORT_STATUS" = "active" ] && [ $x -lt 120 ]
	do
		IMPORT_STATUS=$(aws ec2 describe-import-snapshot-tasks --import-task-ids $IMPORT_TASK_ID --output json | jq -r '.ImportSnapshotTasks[].SnapshotTaskDetail.Status')
		IMPORT_STATUS_MSG=$(aws ec2 describe-import-snapshot-tasks --import-task-ids $IMPORT_TASK_ID --output json | jq -r '.ImportSnapshotTasks[].SnapshotTaskDetail.StatusMessage')
		echo "Import Status: ${IMPORT_STATUS} / ${IMPORT_STATUS_MSG}"
		x=$(( $x + 1 ))
		sleep 15
	done
	if [ $x -eq 120 ]; then
		echo "ERROR: Import task taking too long, exiting..."; exit 1;
	elif [ "$IMPORT_STATUS" = "completed" ]; then
		echo "Import completed Successfully"
	else
		echo "Import Failed, exiting"; exit 2;
	fi

	SNAPSHOT_ID=$(aws ec2 describe-import-snapshot-tasks --import-task-ids $IMPORT_TASK_ID --output json | jq -r '.ImportSnapshotTasks[].SnapshotTaskDetail.SnapshotId')

	aws ec2 wait snapshot-completed --snapshot-ids $SNAPSHOT_ID || exit $?
}

register_image() {
	echo "Registering AMI with Snapshot $SNAPSHOT_ID"

	cat <<EOF > "${AMI_REGISTER_JSON_FILE}"
{
    "Architecture": "x86_64",
    "BlockDeviceMappings": [
        {
            "DeviceName": "/dev/xvda",
            "Ebs": {
                "DeleteOnTermination": true,
                "SnapshotId": "${SNAPSHOT_ID}"
            }
        }
    ],
    "Description": "Peer-pod image",
    "RootDeviceName": "/dev/xvda",
    "VirtualizationType": "hvm",
    "EnaSupport": true
}
EOF

	AMI_ID=$(aws ec2 register-image --name ${AMI_NAME} --cli-input-json="file://${AMI_REGISTER_JSON_FILE}" --output json | jq -r '.ImageId')
	echo "AMI name: ${AMI_NAME}"
	echo "AMI ID: ${AMI_ID}"
}

pre_checks

create_bucket

set_bucket_policies

upload_raw_to_bucket

import_snapshot_n_wait

register_image

