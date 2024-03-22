// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/ibmcloud"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

func CreateConfidentialPodCheckIBMSECommands() []TestCommand {
	testCommands := []TestCommand{
		{
			Command:       []string{"cat", "/sys/firmware/uv/prot_virt_guest"},
			ContainerName: "fakename", //container name will be updated after pod is created.
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				trimmedStdout := strings.Trim(stdout.String(), "\n")
				if trimmedStdout == "1" {
					log.Infof("The pod is SE pod based on content of prot_virt_guest file: %s", stdout.String())
					return true
				} else {
					log.Infof("The pod is non SE pod based on content of prot_virt_guest file: %s", stdout.String())
					return false
				}
			},
		},
		{
			Command:       []string{"grep", "facilities", "/proc/cpuinfo"},
			ContainerName: "fakename", //container name will be updated after pod is created.
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), "158") {
					log.Infof("The pod is SE pod based on facilities of /proc/cpuinfo file: %s", stdout.String())
					return true
				} else {
					log.Infof("The pod is non SE pod based on facilities of /proc/cpuinfo file: %s", stdout.String())
					return false
				}
			},
		},
	}
	return testCommands
}

func NewPodWithPVCFromIBMVPCBlockDriver(namespace, podName, containerName, imageName, csiContainerName, csiImageName string, options ...PodOption) *corev1.Pod {
	runtimeClassName := "kata-remote"
	propagationBidirectional := corev1.MountPropagationBidirectional
	propagationHostPathDirectory := corev1.HostPathDirectory
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name: csiContainerName,
					Env: []corev1.EnvVar{
						{
							Name: "KUBE_NODE_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "ibm-vpc-block-csi-configmap",
								},
							},
						},
					},
					Image:           csiImageName,
					ImagePullPolicy: corev1.PullAlways,
					SecurityContext: &corev1.SecurityContext{
						Privileged:   pointer.Bool(true),
						RunAsNonRoot: pointer.Bool(false),
						RunAsUser:    pointer.Int64(0),
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "healthz",
							ContainerPort: 9808,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:             "kubelet-data-dir",
							MountPath:        "/var/lib/kubelet",
							MountPropagation: &propagationBidirectional,
						},
						{
							Name:      "plugin-dir",
							MountPath: "/tmp",
						},
						{
							Name:      "device-dir",
							MountPath: "/dev",
						},
						{
							Name:      "etcudevpath",
							MountPath: "/etc/udev",
						},
						{
							Name:      "runudevpath",
							MountPath: "/run/udev",
						},
						{
							Name:      "libudevpath",
							MountPath: "/lib/udev",
						},
						{
							Name:      "syspath",
							MountPath: "/sys",
						},
						{
							Name:      "customer-auth",
							MountPath: "/etc/storage_ibmc",
							ReadOnly:  true,
						},
					},
				},
				{
					Name: "csi-podvm-wrapper",
					Env: []corev1.EnvVar{
						{
							Name: "POD_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.name",
								},
							},
						},
						{
							Name: "POD_NAME_SPACE",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						},
						{
							Name: "POD_UID",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.uid",
								},
							},
						},
						{
							Name: "POD_NODE_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
					},
					Args: []string{
						"--v=5",
						"--endpoint=/tmp/csi-podvm-wrapper.sock",
						"--target-endpoint=/tmp/csi.sock",
						"--namespace=kube-system",
					},
					Image:           "quay.io/confidential-containers/csi-podvm-wrapper:latest",
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "plugin-dir",
							MountPath: "/tmp",
						},
					},
				},
				{
					Name:            containerName,
					Image:           imageName,
					ImagePullPolicy: corev1.PullAlways,
					Ports:           []corev1.ContainerPort{{ContainerPort: 80}},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/",
								Port: intstr.FromInt(80),
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       5,
					},
				},
			},
			ServiceAccountName: "ibm-vpc-block-node-sa",
			Volumes: []corev1.Volume{
				{
					Name: "kubelet-data-dir",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/kubelet",
							Type: &propagationHostPathDirectory,
						},
					},
				},
				{
					Name: "plugin-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "device-dir",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/dev",
							Type: &propagationHostPathDirectory,
						},
					},
				},
				{
					Name: "etcudevpath",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/udev",
							Type: &propagationHostPathDirectory,
						},
					},
				},
				{
					Name: "runudevpath",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/run/udev",
							Type: &propagationHostPathDirectory,
						},
					},
				},
				{
					Name: "libudevpath",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/lib/udev",
							Type: &propagationHostPathDirectory,
						},
					},
				},
				{
					Name: "syspath",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/sys",
							Type: &propagationHostPathDirectory,
						},
					},
				},
				{
					Name: "customer-auth",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "storage-secret-store",
						},
					},
				},
			},
		},
	}

	for _, option := range options {
		option(pod)
	}

	return pod
}

// IBMCloudAssert implements the CloudAssert interface for ibmcloud.
type IBMCloudAssert struct {
	VPC *vpcv1.VpcV1
}

func (c IBMCloudAssert) DefaultTimeout() time.Duration {
	return 1 * time.Minute
}

func (c IBMCloudAssert) HasPodVM(t *testing.T, id string) {
	log.Infof("PodVM name: %s", id)
	options := &vpcv1.ListInstancesOptions{}
	instances, _, err := c.VPC.ListInstances(options)

	if err != nil {
		t.Fatal(err)
	}

	for i, instance := range instances.Instances {
		name := *instance.Name
		log.Debugf("Instance number: %d, Instance id: %s, Instance name: %s", i, *instance.ID, name)
		// TODO: PodVM name is podvm-POD_NAME-SANDBOX_ID, where SANDBOX_ID is truncated
		// in the 8th word. Ideally we should match the exact name, not just podvm-POD_NAME.
		if strings.HasPrefix(name, strings.Join([]string{"podvm", id, ""}, "-")) {
			return
		}
	}
	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
}

func (c IBMCloudAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	options := &vpcv1.ListInstancesOptions{}
	instances, _, err := c.VPC.ListInstances(options)

	if err != nil {
		return "", err
	}
	for _, instance := range instances.Instances {
		name := *instance.Name
		if strings.HasPrefix(name, strings.Join([]string{"podvm", podName, ""}, "-")) {
			profile := instance.Profile.Name
			return *profile, nil
		}
	}
	return "", errors.New("Failed to Create PodVM Instance")
}

func GetIBMInstanceProfileType(prefix string, config string) string {
	if strings.EqualFold("s390x", pv.IBMCloudProps.PodvmImageArch) {
		if strings.Contains(pv.IBMCloudProps.InstanceProfile, "e-") {
			return prefix + "z2e-" + config
		} else {
			return prefix + "z2-" + config
		}
	}
	return prefix + "x2-" + config
}

type IBMRollingUpdateAssert struct {
	VPC *vpcv1.VpcV1
	// cache Pod VM instance IDs for rolling update test
	InstanceIDs [2]string
}

func (c *IBMRollingUpdateAssert) CachePodVmIDs(t *testing.T, deploymentName string) {
	options := &vpcv1.ListInstancesOptions{
		VPCID: &pv.IBMCloudProps.VpcID,
	}
	instances, _, err := c.VPC.ListInstances(options)

	if err != nil {
		t.Fatal(err)
	}

	index := 0
	for i, instance := range instances.Instances {
		name := *instance.Name
		log.Debugf("Instance number: %d, Instance id: %s, Instance name: %s", i, *instance.ID, name)
		if strings.Contains(name, deploymentName) {
			c.InstanceIDs[index] = *instance.ID
			index++
		}
	}
}

func (c *IBMRollingUpdateAssert) VerifyOldVmDeleted(t *testing.T) {
	for _, id := range c.InstanceIDs {
		options := &vpcv1.GetInstanceOptions{
			ID: &id,
		}
		in, _, err := c.VPC.GetInstance(options)

		if err != nil {
			log.Printf("Instance %s has been deleted: %v", id, err)
		} else {
			if *in.Status == "deleting" {
				log.Printf("Instance %s is being deleting", id)
			} else {
				log.Printf("Instance %s current status: %s", id, *in.Status)
				t.Fatalf("Instance %s still exists", id)
			}
		}
	}
}
