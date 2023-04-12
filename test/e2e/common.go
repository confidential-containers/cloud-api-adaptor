package e2e

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newPod returns a new Pod object.
func newPod(namespace string, name string, containerName string, runtimeclass string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			Containers:       []corev1.Container{{Name: containerName, Image: "nginx"}},
			DNSPolicy:        "ClusterFirst",
			RestartPolicy:    "Never",
			RuntimeClassName: &runtimeclass,
		},
	}
}

// newPod returns a new Pod object.
func newBusyboxPod(namespace string, name string, containerName string, runtimeclass string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			Containers:       []corev1.Container{{Name: containerName, Image: "quay.io/prometheus/busybox:latest", Command: []string{"/bin/sh", "-c", "sleep 3600"}}},
			DNSPolicy:        "ClusterFirst",
			RuntimeClassName: &runtimeclass,
			RestartPolicy:    corev1.RestartPolicyAlways,
		},
	}
}

func newPodWithConfigMap(namespace string, name string, containerName string, runtimeclass string, configmapname string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			Containers:       []corev1.Container{{Name: containerName, Image: "nginx", VolumeMounts: []corev1.VolumeMount{{Name: "config-volume", MountPath: "/etc/config"}}}},
			DNSPolicy:        "ClusterFirst",
			RestartPolicy:    "Never",
			RuntimeClassName: &runtimeclass,
			Volumes:          []corev1.Volume{{Name: "config-volume", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: configmapname}}}}},
		},
	}
}
func newPodWithSecret(namespace string, name string, containerName string, runtimeclass string, secretname string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			Containers:       []corev1.Container{{Name: containerName, Image: "nginx", VolumeMounts: []corev1.VolumeMount{{Name: "secret-volume", MountPath: "/etc/secret"}}}},
			DNSPolicy:        "ClusterFirst",
			RestartPolicy:    "Never",
			RuntimeClassName: &runtimeclass,
			Volumes:          []corev1.Volume{{Name: "secret-volume", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretname}}}},
		},
	}
}

// newConfigMap returns a new config map object.
func newConfigMap(namespace string, name string, configmapData map[string]string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       configmapData,
	}
}

// newSecret returns a new secret object.
func newSecret(namespace string, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       data,
	}
}

// CloudAssert defines assertions to perform on the cloud provider.
type CloudAssert interface {
	HasPodVM(t *testing.T, id string) // Assert there is a PodVM with `id`.
}
