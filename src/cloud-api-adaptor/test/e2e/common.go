// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	b64 "encoding/base64"
	"net"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const BUSYBOX_IMAGE = "quay.io/prometheus/busybox:latest"
const WAIT_DEPLOYMENT_AVAILABLE_TIMEOUT = time.Second * 180
const DEFAULT_AUTH_SECRET = "auth-json-secret-default"

func isTestWithKbs() bool {
	return os.Getenv("TEST_KBS") == "yes" || os.Getenv("TEST_KBS") == "true"
}

// Setup of Trustee Operator is required for this test
// And is not handled as part of provisioning
func isTestWithTrusteeOperator() bool {
	return os.Getenv("TEST_TRUSTEE_OPERATOR") == "yes"
}

func isTestWithKbsIBMSE() bool {
	return os.Getenv("IBM_SE_CREDS_DIR") != ""
}

func isTestOnCrio() bool {
	return os.Getenv("CONTAINER_RUNTIME") == "crio"
}

func enableAllowAllPodPolicyOverride() bool {
	return os.Getenv("POD_ALLOW_ALL_POLICY_OVERRIDE") == "yes"
}

func encodePolicyFile(policyFilePath string) string {
	policyString, err := os.ReadFile(policyFilePath)
	if err != nil {
		log.Fatal(err)
	}
	return b64.StdEncoding.EncodeToString([]byte(policyString))
}

type PodOption func(*corev1.Pod)

func WithRestartPolicy(restartPolicy corev1.RestartPolicy) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.RestartPolicy = restartPolicy
	}
}

// Optional method to add ContainerPort and ReadinessProbe to listen Port
func WithContainerPort(port int32) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].Ports = []corev1.ContainerPort{{ContainerPort: port}}
		p.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/",
					Port: intstr.FromInt(int(port)),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       5,
		}
	}
}

func WithSecureContainerPort(port int32) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].Ports = append(p.Spec.Containers[0].Ports,
			corev1.ContainerPort{Name: "https", ContainerPort: port})
	}
}

func WithCommand(command []string) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].Command = command
	}
}

func WithEnvironmentalVariables(envVar []corev1.EnvVar) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].Env = envVar
	}
}

func WithImagePullSecrets(secretName string) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: secretName,
			},
		}
	}
}

func WithConfigMapBinding(mountPath string, configMapName string) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].VolumeMounts = append(p.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "config-volume", MountPath: mountPath})
		p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{Name: "config-volume", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: configMapName}}}})
	}
}

func WithSecretBinding(mountPath string, secretName string) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].VolumeMounts = append(p.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "secret-volume", MountPath: mountPath})
		p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{Name: "secret-volume", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}})
	}
}

func WithPVCBinding(mountPath string, pvcName string) PodOption {
	propagationHostToContainer := corev1.MountPropagationHostToContainer
	return func(p *corev1.Pod) {
		p.Spec.Containers[2].VolumeMounts = append(p.Spec.Containers[2].VolumeMounts, corev1.VolumeMount{Name: "pvc-volume", MountPath: mountPath, MountPropagation: &propagationHostToContainer})
		p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{Name: "pvc-volume", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}})
	}
}

func WithAnnotations(data map[string]string) PodOption {
	return func(p *corev1.Pod) {
		p.ObjectMeta.Annotations = data
	}
}

func WithLabel(data map[string]string) PodOption {
	return func(p *corev1.Pod) {
		p.ObjectMeta.Labels = data
	}
}

// Option to handle SecurityContext
func WithSecurityContext(sc *corev1.SecurityContext) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].SecurityContext = sc
	}
}

// Option to add InitContainers
func WithInitContainers(initContainers []corev1.Container) PodOption {
	return func(p *corev1.Pod) {
		p.Spec.InitContainers = initContainers
	}
}

func NewPod(namespace string, podName string, containerName string, imageName string, options ...PodOption) *corev1.Pod {
	runtimeClassName := "kata-remote"
	annotationData := map[string]string{}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName,
			Namespace:   namespace,
			Annotations: annotationData},
		Spec: corev1.PodSpec{
			Containers:       []corev1.Container{{Name: containerName, Image: imageName, ImagePullPolicy: corev1.PullAlways}},
			RuntimeClassName: &runtimeClassName,
		},
	}

	for _, option := range options {
		option(pod)
	}

	// Don't override the policy annotation if it's already set
	if enableAllowAllPodPolicyOverride() {
		allowAllPolicyFilePath := "fixtures/policies/allow-all.rego"
		if _, ok := pod.ObjectMeta.Annotations["io.katacontainers.config.agent.policy"]; !ok {
			pod.ObjectMeta.Annotations["io.katacontainers.config.agent.policy"] = encodePolicyFile(allowAllPolicyFilePath)
		}
	}

	return pod
}

func NewBusyboxPod(namespace string) *corev1.Pod {
	return NewBusyboxPodWithName(namespace, "busybox")
}

func NewPrivPod(namespace string, podName string) *corev1.Pod {
	sc := &corev1.SecurityContext{
		Privileged: func(b bool) *bool { return &b }(true),
	}
	return NewPod(namespace, podName, "busybox", BUSYBOX_IMAGE, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithSecurityContext(sc))
}

// Method to create a Pod with initContainer
func NewPodWithInitContainer(namespace string, podName string) *corev1.Pod {

	initContainer := []corev1.Container{
		{
			Name:    "init-container",
			Image:   BUSYBOX_IMAGE,
			Command: []string{"/bin/sh", "-c", "echo 'init container'"},
		},
	}

	return NewPod(namespace, podName, "busybox", BUSYBOX_IMAGE, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithInitContainers(initContainer))
}

func NewBusyboxPodWithName(namespace, podName string) *corev1.Pod {
	return NewPod(namespace, podName, "busybox", BUSYBOX_IMAGE, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
}

func NewPodWithPolicy(namespace, podName, policyFilePath string) *corev1.Pod {
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	annotationData := map[string]string{
		"io.katacontainers.config.agent.policy": encodePolicyFile(policyFilePath),
	}
	return NewPod(namespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
}

// NewConfigMap returns a new config map object.
func NewConfigMap(namespace, name string, configMapData map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       configMapData,
	}
}

// NewSecret returns a new secret object.
func NewSecret(namespace, name string, data map[string][]byte, secretType corev1.SecretType) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       data,
		Type:       secretType,
	}
}

// NewJob returns a new job
func NewJob(namespace, name string) *batchv1.Job {
	runtimeClassName := "kata-remote"
	BackoffLimit := int32(8)
	TerminateGracePeriod := int64(0)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &TerminateGracePeriod,
					Containers: []corev1.Container{{
						Name:    name,
						Image:   "quay.io/prometheus/busybox:latest",
						Command: []string{"/bin/sh", "-c", "echo 'scale=5; 4*a(1)' | bc -l"},
					}},
					RestartPolicy:    corev1.RestartPolicyNever,
					RuntimeClassName: &runtimeClassName,
				},
			},
			BackoffLimit: &BackoffLimit,
		},
	}
}

// NewPVC returns a new pvc object.
func NewPVC(namespace, name, storageClassName, diskSize string, accessModel corev1.PersistentVolumeAccessMode) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				accessModel,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(diskSize),
				},
			},
		},
	}
}

func NewService(namespace, serviceName string, servicePorts []corev1.ServicePort, labels map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports:    servicePorts,
			Selector: labels,
		},
	}
}

func WaitForClusterIP(t *testing.T, client klient.Client, svc *v1.Service) string {
	var clusterIP string
	if err := wait.For(conditions.New(client.Resources()).ResourceMatch(svc, func(object k8s.Object) bool {
		svcObj, ok := object.(*v1.Service)
		if !ok {
			log.Printf("Not a Service object: %v", object)
			return false
		}
		clusterIP = svcObj.Spec.ClusterIP
		ip := net.ParseIP(clusterIP)
		if ip != nil {
			return true
		} else {
			log.Printf("Current service: %v", svcObj)
			return false
		}
	}), wait.WithTimeout(WAIT_DEPLOYMENT_AVAILABLE_TIMEOUT)); err != nil {
		t.Fatal(err)
	}

	return clusterIP
}

// CloudAssert defines assertions to perform on the cloud provider.
type CloudAssert interface {
	HasPodVM(t *testing.T, id string)                             // Assert there is a PodVM with `id`.
	GetInstanceType(t *testing.T, podName string) (string, error) // Get Instance Type of PodVM
	DefaultTimeout() time.Duration                                // Default timeout for cloud operations
}

// RollingUpdateAssert defines assertions for rolling update test
type RollingUpdateAssert interface {
	CachePodVmIDs(t *testing.T, deploymentName string) // Cache Pod VM IDs before rolling update
	VerifyOldVmDeleted(t *testing.T)                   // Verify old Pod VMs have been deleted
}
