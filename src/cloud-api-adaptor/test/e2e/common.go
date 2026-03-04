// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/initdata"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"

	"text/template"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const WAIT_DEPLOYMENT_AVAILABLE_TIMEOUT = time.Second * 180
const INITDATA_ANNOTATION = "io.katacontainers.config.hypervisor.cc_init_data"

const POLICY = `package agent_policy

import future.keywords.in
import future.keywords.every

import input

# Default values, returned by OPA when rules cannot be evaluated to true.
default CopyFileRequest := true
default CreateContainerRequest := true
default CreateSandboxRequest := true
default DestroySandboxRequest := true
default ExecProcessRequest := true
default GetOOMEventRequest := true
default GuestDetailsRequest := true
default OnlineCPUMemRequest := true
default PullImageRequest := true
default ReadStreamRequest := true
default RemoveContainerRequest := true
default RemoveStaleVirtiofsShareMountsRequest := true
default SignalProcessRequest := true
default StartContainerRequest := true
default StatsContainerRequest := true
default TtyWinResizeRequest := true
default UpdateEphemeralMountsRequest := true
default UpdateInterfaceRequest := true
default UpdateRoutesRequest := true
default WaitProcessRequest := true
default WriteStreamRequest := true
`

type initdataParams struct {
	CoCoASURL string
	KBSURL    string
	KBSCert   string
	Policy    string
}

var testInitdataTmpl string = `algorithm = "sha384"
version = "0.1.0"

[data]
"aa.toml" = '''
[token_configs]
[token_configs.coco_as]
url = '{{ .CoCoASURL }}'

[token_configs.kbs]
url = '{{ .KBSURL }}'
{{- if .KBSCert }}
cert = """{{ .KBSCert }}"""
{{- end }}
'''

"cdh.toml"  = '''
socket = 'unix:///run/confidential-containers/cdh.sock'

[kbc]
name = 'cc_kbc'
url = '{{ .KBSURL }}'
{{- if .KBSCert }}
kbs_cert = """{{ .KBSCert }}"""
{{- end }}
'''
{{ if .Policy }}

"policy.rego" = '''{{ .Policy }}'''
{{- end }}
`

// Build gzipped and base64 encoded string
func buildInitdataAnnotation(kbsEndpoint string) (string, error) {
	params := initdataParams{
		CoCoASURL: kbsEndpoint,
		KBSURL:    kbsEndpoint,
		Policy:    POLICY,
	}

	if strings.HasPrefix(kbsEndpoint, "https") {
		content, err := os.ReadFile("../trustee/kbs/config/kubernetes/base/https-cert.pem")
		if err != nil {
			return "", err
		}
		params.KBSCert = string(content)
	}

	tmpl, err := template.New("initdata").Parse(testInitdataTmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	initdataToml := buf.String()
	return initdata.Encode(initdataToml)
}

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

// Convenience method as we follow this pattern a lot
func getBusyboxTestImage(t *testing.T) string {
	image, err := utils.GetImage("busybox")
	if err != nil {
		t.Fatal(err)
	}
	return image
}

func encodePolicyFile(policyFilePath string) string {
	policyString, err := os.ReadFile(policyFilePath)
	if err != nil {
		log.Fatal(err)
	}
	return b64.StdEncoding.EncodeToString([]byte(policyString))
}

type PodOption func(*corev1.Pod)

func GetContainerReference(pod *corev1.Pod, containerName string) *corev1.Container {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			return &pod.Spec.Containers[i]
		}
	}
	log.Error(fmt.Sprintf("Failed getting container reference for container %s", containerName))
	return nil
}

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

func WithCpuMemRequestAndLimit(cpuRequest, memRequest, cpuLimit, memLimit string) PodOption {
	// If any of the parameters is empty, don't set it
	return func(p *corev1.Pod) {
		p.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}
		if cpuRequest != "" {
			p.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse(cpuRequest)
		}
		if memRequest != "" {
			p.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse(memRequest)
		}
		if cpuLimit != "" {
			p.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse(cpuLimit)
		}
		if memLimit != "" {
			p.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory] = resource.MustParse(memLimit)
		}
	}
}

type JobOption func(*batchv1.Job)

func WithJobCommand(command []string) JobOption {
	return func(j *batchv1.Job) {
		j.Spec.Template.Spec.Containers[0].Command = command
	}
}

func WithJobAnnotations(data map[string]string) JobOption {
	return func(j *batchv1.Job) {
		j.Spec.Template.ObjectMeta.Annotations = data
	}
}

func WithEnvironmentVariables(envVar []corev1.EnvVar) PodOption {
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

func WithSecretBinding(t *testing.T, mountPath string, secretName string, containerName string) PodOption {
	return func(p *corev1.Pod) {
		if c := GetContainerReference(p, containerName); c != nil {
			c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: "secret-volume-" + containerName, MountPath: mountPath})
		} else {
			t.Fatalf("container %q not found in pod spec", containerName)
		}
		p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{Name: "secret-volume-" + containerName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}})
	}
}

func WithPVCBinding(t *testing.T, mountPath string, pvcName string, containerName string) PodOption {
	propagationHostToContainer := corev1.MountPropagationHostToContainer
	return func(p *corev1.Pod) {
		if c := GetContainerReference(p, containerName); c != nil {
			c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: "pvc-volume-" + containerName, MountPath: mountPath, MountPropagation: &propagationHostToContainer})
		} else {
			t.Fatalf("container %q not found in pod spec", containerName)
		}
		p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{Name: "pvc-volume-" + containerName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}})
	}
}

func WithInitdata(kbsEndpoint string) PodOption {
	return func(p *corev1.Pod) {
		if p.ObjectMeta.Annotations == nil {
			p.ObjectMeta.Annotations = make(map[string]string)
		}
		key := INITDATA_ANNOTATION
		value, err := buildInitdataAnnotation(kbsEndpoint)
		if err != nil {
			log.Fatalf("failed to build initdata %s", err)
		}
		p.ObjectMeta.Annotations[key] = value
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

type PVCOption func(*corev1.PersistentVolumeClaim)

// Option to add StorageClass
func WithStorageClass(sc string) PVCOption {
	return func(pvc *corev1.PersistentVolumeClaim) {
		pvc.Spec.StorageClassName = &sc
	}
}

// Option to add Persistent volume
func WithPersistentVolume(pvName string) PVCOption {
	return func(pvc *corev1.PersistentVolumeClaim) {
		pvc.Spec.VolumeName = pvName
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

// Struct for convenience
type PodOrError struct {
	pod *corev1.Pod
	err error
}

func fromError(err error) PodOrError {
	return PodOrError{pod: nil, err: err}
}

func fromPod(pod *corev1.Pod) PodOrError {
	return PodOrError{pod: pod, err: nil}
}

func (p PodOrError) GetPodOrFatal(t *testing.T) *corev1.Pod {
	if p.err != nil {
		t.Fatal(p.err)
	}
	return p.pod
}

func NewBusyboxPod(namespace string) PodOrError {
	return NewBusyboxPodWithName(namespace, "busybox")
}

func NewPrivPod(namespace string, podName string) PodOrError {
	sc := &corev1.SecurityContext{
		Privileged: func(b bool) *bool { return &b }(true),
	}
	busyboxImage, err := utils.GetImage("busybox")
	if err != nil {
		return fromError(err)
	}
	return fromPod(NewPod(namespace, podName, "busybox", busyboxImage, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithSecurityContext(sc)))
}

// Method to create a Pod with initContainer
func NewPodWithInitContainer(namespace string, podName string) PodOrError {
	busyboxImage, err := utils.GetImage("busybox")
	if err != nil {
		return fromError(err)
	}
	initContainer := []corev1.Container{
		{
			Name:    "init-container",
			Image:   busyboxImage,
			Command: []string{"/bin/sh", "-c", "echo 'init container'"},
		},
	}

	return fromPod(NewPod(namespace, podName, "busybox", busyboxImage, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithInitContainers(initContainer)))
}

func NewBusyboxPodWithName(namespace, podName string) PodOrError {
	busyboxImage, err := utils.GetImage("busybox")
	if err != nil {
		return fromError(err)
	}
	return fromPod(NewPod(namespace, podName, "busybox", busyboxImage, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"})))
}

func NewBusyboxPodWithNameWithInitdata(namespace, podName string, kbsEndpoint string) PodOrError {
	initdata, err := buildInitdataAnnotation(kbsEndpoint)
	if err != nil {
		return fromError(err)
	}
	annotationData := map[string]string{
		INITDATA_ANNOTATION: initdata,
	}
	busyboxImage, err := utils.GetImage("busybox")
	if err != nil {
		return fromError(err)
	}
	return fromPod(NewPod(namespace, podName, "busybox", busyboxImage, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData)))
}

func NewPodWithPolicy(namespace, podName, policyFilePath string) PodOrError {
	containerName := "busybox"
	imageName, err := utils.GetImage("busybox")
	if err != nil {
		return fromError(err)
	}
	annotationData := map[string]string{
		"io.katacontainers.config.agent.policy": encodePolicyFile(policyFilePath),
	}
	return fromPod(NewPod(namespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData)))
}

// NewConfigMap returns a new config map object.
func NewConfigMap(namespace, name string, configMapData map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       configMapData,
	}
}

// NewImagePullSecret returns a new imagePull secret object.
func NewImagePullSecret(namespace, name string, image string, credentials string) *corev1.Secret {
	registryName := strings.Split(image, "/")[0]
	template := `{
	"auths": {
		"%s": {
			"auth": "%s"
		}
	}
}`
	authJSON := fmt.Sprintf(template, registryName, credentials)
	secretData := map[string][]byte{v1.DockerConfigJsonKey: []byte(authJSON)}

	return NewSecret(namespace, name, secretData, v1.SecretTypeDockerConfigJson)
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
func NewJob(namespace, name string, backoffLimit int32, image string, options ...JobOption) *batchv1.Job {
	command := []string{"/bin/sh", "-c", "echo 'scale=5; 4*a(1)' | bc -l"}

	runtimeClassName := "kata-remote"
	TerminateGracePeriod := int64(0)
	job := batchv1.Job{
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
						Image:   image,
						Command: command,
					}},
					RestartPolicy:    corev1.RestartPolicyNever,
					RuntimeClassName: &runtimeClassName,
				},
			},
			BackoffLimit: &backoffLimit,
		},
	}

	for _, option := range options {
		option(&job)
	}

	return &job
}

// NewPVC returns a new pvc object.
func NewPVC(namespace, name, diskSize string, accessModel corev1.PersistentVolumeAccessMode, opts ...PVCOption) *corev1.PersistentVolumeClaim {

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				accessModel,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(diskSize),
				},
			},
		},
	}
	for _, opt := range opts {
		opt(pvc)
	}
	return pvc
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

func WaitForPVCBound(client klient.Client, pvc *v1.PersistentVolumeClaim, timeout time.Duration) error {
	return wait.For(conditions.New(client.Resources()).ResourceMatch(pvc, func(object k8s.Object) bool {
		return object.(*v1.PersistentVolumeClaim).Status.Phase == v1.ClaimBound
	}), wait.WithTimeout(timeout))
}

func CreateSealedSecretValue(resourceURI string) string {
	metadata := map[string]interface{}{
		"version":           "0.1.0",
		"type":              "vault",
		"name":              resourceURI,
		"provider":          "kbs",
		"provider_settings": map[string]interface{}{},
		"annotations":       map[string]interface{}{},
	}
	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		panic(err)
	}
	payload := b64.RawURLEncoding.EncodeToString([]byte(metadataStr))
	header := "fakejwsheader"
	signature := "fakesignature"
	return fmt.Sprintf("sealed.%s.%s.%s", header, payload, signature)
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
