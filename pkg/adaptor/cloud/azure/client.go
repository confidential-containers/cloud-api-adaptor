// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

func NewAzureClient(config Config) (azcore.TokenCredential, error) {

	cred, err := azidentity.NewClientSecretCredential(config.TenantId, config.ClientId, config.ClientSecret, nil)
	if err != nil {
		return nil, err
	}
	//azidentity.ClientSecretCredential
	return cred, nil

}
