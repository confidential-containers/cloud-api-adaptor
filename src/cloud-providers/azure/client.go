// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

func NewAzureClient(config Config) (azcore.TokenCredential, error) {
	// Use workload identity if the client secret is empty.
	if config.ClientSecret == "" {
		logger.Printf("using workload identity")
		return azidentity.NewWorkloadIdentityCredential(nil)
	}

	return azidentity.NewClientSecretCredential(config.TenantID, config.ClientID, config.ClientSecret, nil)
}
