//go:build azure

package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
)


func CreateInstance(c context.Context, cred azcore.TokenCredential, input *armcompute.VirtualMachine) error {
	return nil
}

func DeleteInstance(c context.Context, cred azcore.TokenCredential, id string) error {
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
