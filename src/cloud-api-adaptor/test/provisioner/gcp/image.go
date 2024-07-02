// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"google.golang.org/api/compute/v1"
	// pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	// log "github.com/sirupsen/logrus"
)

// AMIImage represents an AMI image
type GCPImage struct {
	Service     *compute.Service
	Description string
	Name        string
}

func NewGCPImage(srv *compute.Service, name string) (*GCPImage, error) {
	return &GCPImage{
		Service:     srv,
		Description: "Peer Pod VM image",
		Name:        name,
	}, nil
}

// // importEBSSnapshot Imports the disk image into the EBS
// func (i *AMIImage) importEBSSnapshot(bucket *S3Bucket) error {
// 	// Create the import snapshot task
// 	importSnapshotOutput, err := i.Client.ImportSnapshot(context.TODO(), &ec2.ImportSnapshotInput{
// 		Description: aws.String("Peer Pod VM disk snapshot"),
// 		DiskContainer: &ec2types.SnapshotDiskContainer{
// 			Description: aws.String(i.DiskDescription),
// 			Format:      aws.String(i.DiskFormat),
// 			UserBucket: &ec2types.UserBucket{
// 				S3Bucket: aws.String(bucket.Name),
// 				S3Key:    aws.String(bucket.Key),
// 			},
// 		},
// 	})
// 	if err != nil {
// 		return err
// 	}
//
// 	//taskId := *importSnapshotOutput.ImportTaskId
// 	describeTasksInput := &ec2.DescribeImportSnapshotTasksInput{
// 		ImportTaskIds: []string{*importSnapshotOutput.ImportTaskId},
// 	}
//
// 	// Wait the import task to finish
// 	waiter := ec2.NewSnapshotImportedWaiter(i.Client)
// 	if err = waiter.Wait(context.TODO(), describeTasksInput, time.Minute*3); err != nil {
// 		return err
// 	}
//
// 	// Finally get the snapshot ID
// 	describeTasks, err := i.Client.DescribeImportSnapshotTasks(context.TODO(), describeTasksInput)
// 	if err != nil {
// 		return err
// 	}
// 	taskDetail := describeTasks.ImportSnapshotTasks[0].SnapshotTaskDetail
// 	i.EBSSnapshotId = *taskDetail.SnapshotId
//
// 	return nil
// }
//
// // registerImage Registers an AMI image
// func (i *AMIImage) registerImage(imageName string) error {
//
// 	if i.EBSSnapshotId == "" {
// 		return fmt.Errorf("EBS Snapshot ID not found\n")
// 	}
//
// 	result, err := i.Client.RegisterImage(context.TODO(), &ec2.RegisterImageInput{
// 		Name:         aws.String(imageName),
// 		Architecture: ec2types.ArchitectureValuesX8664,
// 		BlockDeviceMappings: []ec2types.BlockDeviceMapping{{
// 			DeviceName: aws.String(i.RootDeviceName),
// 			Ebs: &ec2types.EbsBlockDevice{
// 				DeleteOnTermination: aws.Bool(true),
// 				SnapshotId:          aws.String(i.EBSSnapshotId),
// 			},
// 		}},
// 		Description:        aws.String(i.Description),
// 		EnaSupport:         aws.Bool(true),
// 		RootDeviceName:     aws.String(i.RootDeviceName),
// 		VirtualizationType: aws.String("hvm"),
// 	})
// 	if err != nil {
// 		return err
// 	}
//
// 	// Save the AMI ID
// 	i.ID = *result.ImageId
// 	return nil
// }
//
// // uploadLargeFileWithCli Uploads large files (>5GB) using the AWS CLI
// func (b *S3Bucket) uploadLargeFileWithCli(filepath string) error {
// 	file, err := os.Open(filepath)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()
//
// 	stat, err := file.Stat()
// 	if err != nil {
// 		return err
// 	}
// 	key := stat.Name()
// 	defer func() {
// 		if err == nil {
// 			b.Key = key
// 		}
// 	}()
//
// 	s3uri := "s3://" + b.Name + "/" + key
//
// 	// TODO: region!
// 	cmd := exec.Command("aws", "s3", "cp", filepath, s3uri)
// 	out, err := cmd.CombinedOutput()
// 	fmt.Printf("%s\n", out)
// 	if err != nil {
// 		return err
// 	}
//
// 	return nil
// }
//
// // ConvertQcow2ToRaw Converts an qcow2 image to raw. Requires `qemu-img` installed.
// func ConvertQcow2ToRaw(qcow2 string, raw string) error {
// 	cmd := exec.Command("qemu-img", "convert", "-O", "raw", qcow2, raw)
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr
// 	err := cmd.Run()
// 	if err != nil {
// 		return err
// 	}
//
// 	return nil
// }
