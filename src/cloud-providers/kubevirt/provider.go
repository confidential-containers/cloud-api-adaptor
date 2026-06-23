// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package kubevirt

import (
	"context"
	"log"
	"net/netip"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// Initialize logger for Kubevirt provider
var logger = log.New(log.Writer(), "[adaptor/cloud/kubevirt] ", log.LstdFlags|log.Lmsgprefix)

// Maximum length for instance names
const maxInstanceNameLen = 253

// KubevirtProvider implements the Provider interface for Kubevirt
type kubevirtProvider struct {
	kubevirtClient   *Virtclient
	kubernetesClient *K8sclient
	serviceConfig    *Config
	vmtemplate       *kubevirtv1.VirtualMachine
	servicetemplate  *corev1.Service
}

// NewProvider creates a new Kubevirt provider.
func NewProvider(config *Config) (provider.Provider, error) {
	kubevirtClient, err := NewProviderClient()
	if err != nil {
		logger.Printf("Unable to create KubeVirt client: %v", err)
		return nil, err
	}

	kubernetesClient, err := NewKubernetesClient()
	if err != nil {
		logger.Printf("Unable to create Kubernetes client: %v", err)
		return nil, err
	}

	vm, err := VMconfigUnmarshal()
	if err != nil {
		logger.Printf("Failed to unmarshal VMconfig: %v", err)
		return nil, err
	}

	var service *corev1.Service
	if config.serviceconfigfile != "" {
		service, err = ServiceconfigUnmarshal(config.serviceconfigfile)
		if err != nil {
			logger.Printf("Failed to unmarshal Serviceconfig: %v", err)
			return nil, err
		}
	}

	return &kubevirtProvider{
		kubevirtClient:   kubevirtClient,
		kubernetesClient: kubernetesClient,
		serviceConfig:    config,
		vmtemplate:       vm,
		servicetemplate:  service,
	}, nil
}

// Create a new VirtualMachine. Additionally, create a Service using the optional feature.
func (p *kubevirtProvider) CreateInstance(ctx context.Context, podname, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {
	instancename := util.GenerateInstanceName(podname, sandboxID, maxInstanceNameLen)

	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		logger.Printf("Failed to cloudConfig Generate :%v", err)
		return nil, err
	}

	vm := p.vmtemplate.DeepCopy()

	_, err = p.kubernetesClient.CreateSecret(ctx, vm.Namespace, instancename, cloudConfigData)
	if err != nil {
		logger.Printf("Failed to create cloud-init Secret: %v", err)
		return nil, err
	}

	logger.Printf("Successfully create Secret")

	createvm, err := p.kubevirtClient.CreateVM(ctx, vm, instancename)
	if err != nil {
		logger.Printf("Failed to CreateVM %s: %v", instancename, err)
		return nil, err
	}

	createvmi, err := p.kubevirtClient.GetPodVM(ctx, createvm.Namespace, createvm.Name)
	if err != nil {
		logger.Printf("Failed to GetPodVM %s: %v", instancename, err)
		return nil, err
	}

	var ips []netip.Addr
	if createvmi != nil {
		for _, iface := range createvmi.Status.Interfaces {
			if len(iface.IP) > 0 {
				addr, err := netip.ParseAddr(iface.IP)
				if err != nil {
					logger.Printf("Failed to parse IP address %s: %v", iface.IP, err)
				}
				ips = append(ips, addr)
			}
		}
	}

	if p.serviceConfig.serviceconfigfile != "" {
		service := p.servicetemplate.DeepCopy()

		createservice, err := p.kubernetesClient.CreateService(ctx, service.Namespace, service)
		if err != nil {
			logger.Printf("Failed to create Service %s: %v", service.Name, err)
			return nil, err
		}

		if createservice.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, ingress := range createservice.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					addr, err := netip.ParseAddr(ingress.IP)
					if err != nil {
						logger.Printf("Failed to parse IP address %s: %v", ingress.IP, err)
					}
					ips = append(ips, addr)
				}
			}
		}
	}

	instance := &provider.Instance{
		ID:   string(createvm.UID),
		Name: createvm.Name,
		IPs:  ips,
	}

	logger.Printf("Successfully create instance: %s", createvm.Name)

	return instance, nil
}

// Delete the created VM. Also delete the Service created with the optional feature.
func (p *kubevirtProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	logger.Printf("Deleting instance: %s", instanceID)

	getvm, err := p.kubevirtClient.GetVM(ctx, p.vmtemplate.Namespace, instanceID)
	if err != nil {
		logger.Printf("Failed to get VM %s: %v", instanceID, err)
		return err
	}

	err = p.kubevirtClient.DeleteVM(ctx, getvm.Namespace, getvm.Name)
	if err != nil {
		logger.Printf("Failed to delete VM %s: %v", getvm.Name, err)
		return err
	}

	if p.serviceConfig.serviceconfigfile != "" {
		err = p.kubernetesClient.DeleteService(ctx, p.servicetemplate.Namespace, p.servicetemplate.Name)
		if err != nil {
			logger.Printf("Failed to delete Service %s: %v", p.servicetemplate.Name, err)
			return err
		}
	}

	err = p.kubernetesClient.DeleteSecret(ctx, p.vmtemplate.Namespace, getvm.Name)
	if err != nil {
		logger.Printf("Failed to delete cloud-init Secret: %v", err)
		return err
	}

	logger.Printf("Successfully sent delete request for instance: %s", instanceID)
	return nil
}

func (p *kubevirtProvider) Teardown() error {
	return nil
}

func (p *kubevirtProvider) ConfigVerifier() error {
	return nil
}
