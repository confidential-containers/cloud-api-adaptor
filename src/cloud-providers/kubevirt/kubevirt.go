// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package kubevirt

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	"sigs.k8s.io/yaml"
)

const (
	kubeconfigpath = "/etc/config/caa/kubevirt/kubeconfig"
	vmconfigpath   = "/etc/config/caa/kubevirt/podvm.yaml"
	cloudinitName  = "cloudinit"
)

type Virtclient struct {
	client kubecli.KubevirtClient
}

type K8sclient struct {
	client *kubernetes.Clientset
}

// NewProviderClient creates a new Kubevirt provider client with authentication
func NewProviderClient() (*Virtclient, error) {

	virtClient, err := kubecli.GetKubevirtClientFromFlags("", kubeconfigpath)
	if err != nil {
		return nil, fmt.Errorf("Failed to create KubeVirt client: %w", err)
	}

	fmt.Println("Successfully connected to KubeVirt API!")

	return &Virtclient{client: virtClient}, nil
}

// NewKubernetesClient creates a new Kubernetes client
func NewKubernetesClient() (*K8sclient, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigpath)

	if err != nil {
		return nil, fmt.Errorf("Failed to BuildConfigFromFlags: %w", err)
	}
	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kubernetes client: %w", err)
	}
	return &K8sclient{client: k8sClient}, nil
}

// CreateVM creates a new VirtualMachine
func (c *Virtclient) CreateVM(vm *kubevirtv1.VirtualMachine, vmname string) (*kubevirtv1.VirtualMachine, error) {
	vm.Name = vmname

	cloudInitSource := kubevirtv1.CloudInitNoCloudSource{
		UserDataSecretRef: &corev1.LocalObjectReference{
			Name: secretName(vmname),
		},
	}

	cloudInitVolume := kubevirtv1.Volume{
		Name: cloudinitName,
		VolumeSource: kubevirtv1.VolumeSource{
			CloudInitNoCloud: &cloudInitSource,
		},
	}

	vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, cloudInitVolume)

	cloudInitDisk := kubevirtv1.Disk{
		Name: cloudinitName,
		DiskDevice: kubevirtv1.DiskDevice{
			Disk: &kubevirtv1.DiskTarget{
				Bus: kubevirtv1.DiskBusVirtio,
			},
		},
	}

	vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, cloudInitDisk)

	createvm, err := c.client.VirtualMachine(vm.Namespace).Create(context.Background(), vm, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to create VirtualMachine: %w", err)
	}
	return createvm, nil
}

// Used to retrieve PodVM information after the VirtualMachine is launched.
func (c *Virtclient) GetPodVM(namespace string, vmname string) (*kubevirtv1.VirtualMachineInstance, error) {
	var currentvmi *kubevirtv1.VirtualMachineInstance

	err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 5*time.Minute, false, func(ctx context.Context) (bool, error) {
		vmi, err := c.client.VirtualMachineInstance(namespace).Get(ctx, vmname, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if vmi != nil && len(vmi.Status.Interfaces) > 0 {
			for _, iface := range vmi.Status.Interfaces {
				if len(iface.IP) > 0 {
					currentvmi = vmi
					return true, nil
				}
			}
		}
		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to get VirtualMachineInstance: %w", err)
	}

	return currentvmi, nil
}

// Build the service and return its information.
func (c *K8sclient) Getservice(namespace string, service *corev1.Service) (*corev1.Service, error) {
	createservice, err := c.client.CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to create Service: %w", err)
	}
	return createservice, nil
}

// GetVM verifies that the VirtualMachine to be deleted exists.
func (c *Virtclient) GetVM(namespace string, targetUID string) (*kubevirtv1.VirtualMachine, error) {
	vmlist, err := c.client.VirtualMachine(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to list VMs in Namespace '%s': %w", namespace, err)
	}
	for _, vm := range vmlist.Items {
		if string(vm.UID) == targetUID {
			return &vm, nil
		}
	}
	return nil, fmt.Errorf("VM with UID %s not found", targetUID)
}

// DeleteVM deletes the target VirtualMachine.
func (c *Virtclient) DeleteVM(namespace string, vmname string) error {
	err := c.client.VirtualMachine(namespace).Delete(context.Background(), vmname, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete VM: %w", err)
	}
	return nil
}

// Delete the target service.
func (c *K8sclient) DeleteService(namespace string, servicename string) error {
	err := c.client.CoreV1().Services(namespace).Delete(context.TODO(), servicename, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete Service: %w", err)
	}
	return nil
}

// CreateSecret creates a new Secret in the specified namespace.
func (c *K8sclient) CreateSecret(namespace string, vmname string, cloudConfigData string) (*corev1.Secret, error) {

	secretData := map[string][]byte{
		"userdata": []byte(cloudConfigData),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName(vmname),
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	createdSecret, err := c.client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to create Secret: %w", err)
	}
	return createdSecret, nil
}

// DeleteSecret delete a Secret in the specified namespace.
func (c *K8sclient) DeleteSecret(namespace string, vmname string) error {
	err := c.client.CoreV1().Secrets(namespace).Delete(context.TODO(), secretName(vmname), metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete Secret: %w", err)
	}
	return nil
}

// Read the YAML file containing the VM information and unmarshal it.
func VMconfigUnmarshal() (*kubevirtv1.VirtualMachine, error) {
	vmfile, err := os.ReadFile(vmconfigpath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read VMfile path=%s: %v", vmconfigpath, err)
	}

	vm := &kubevirtv1.VirtualMachine{}
	err = yaml.Unmarshal(vmfile, vm)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal VMfile %s: %v", vmconfigpath, err)
	}
	return vm, nil
}

// Read the YAML file containing the Service information and unmarshal it.
func ServiceconfigUnmarshal(servicefilepath string) (*corev1.Service, error) {
	servicefile, err := os.ReadFile(servicefilepath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read Servicefile path=%s: %v", servicefilepath, err)
	}

	service := &corev1.Service{}
	err = yaml.Unmarshal(servicefile, service)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal Servicefile %s: %v", servicefilepath, err)
	}
	return service, nil
}

// Create a Secret name to be used for VM creation.
func secretName(vmname string) string {
	return vmname + "-secret"
}
