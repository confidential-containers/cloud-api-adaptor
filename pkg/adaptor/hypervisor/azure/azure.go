//go:build azure

package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
)

func CreateInstance(ctx context.Context, s *hypervisorService, parameters *armcompute.VirtualMachine) (*armcompute.VirtualMachine, error) {
	vmClient, err := armcompute.NewVirtualMachinesClient(s.serviceConfig.SubscriptionId, s.azureClient, nil)
	if err != nil {
		return nil, fmt.Errorf("creating VM client: %w", err)
	}

	vmName := *parameters.Properties.OSProfile.ComputerName

	pollerResponse, err := vmClient.BeginCreateOrUpdate(ctx, s.serviceConfig.ResourceGroupName, vmName, *parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning VM creation or update: %w", err)
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("waiting for the VM creation: %w", err)
	}

	logger.Printf("created VM successfully: %s", *resp.ID)

	return &resp.VirtualMachine, nil
}

func DeleteInstance(ctx context.Context, s *hypervisorService, vmName string) error {
	vmClient, err := armcompute.NewVirtualMachinesClient(s.serviceConfig.SubscriptionId, s.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating VM client: %w", err)
	}

	pollerResponse, err := vmClient.BeginDelete(ctx, s.serviceConfig.ResourceGroupName, vmName, nil)
	if err != nil {
		return fmt.Errorf("beginning VM deletion: %w", err)
	}

	if _, err = pollerResponse.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("waiting for the VM deletion: %w", err)
	}

	logger.Printf("deleted VM successfully: %s", vmName)

	return nil
}

func NewAzureClient(cloudCfg Config) (azcore.TokenCredential, error) {

	cred, err := azidentity.NewClientSecretCredential(cloudCfg.TenantId, cloudCfg.ClientId, cloudCfg.ClientSecret, nil)
	if err != nil {
		return nil, err
	}
	//azidentity.ClientSecretCredential
	return cred, nil

}
