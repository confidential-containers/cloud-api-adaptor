// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloudpowervs

import (
	"context"

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/iamidentityv1"
)

type powervsService struct {
	session           *ibmpisession.IBMPISession
	serviceInstanceID string
}

func newPowervsClient(apikey, serviceinstanceID, zone string) (*powervsService, error) {
	options := &ibmpisession.IBMPIOptions{}
	options.Authenticator = &core.IamAuthenticator{
		ApiKey: apikey,
	}
	ic, err := newIdentityClient(options.Authenticator)
	if err != nil {
		return nil, err
	}

	account, err := getAccount(apikey, ic)
	if err != nil {
		return nil, err
	}
	options.UserAccount = *account
	options.Zone = zone

	piSession, err := ibmpisession.NewIBMPISession(options)
	if err != nil {
		return nil, err
	}

	return &powervsService{
		session:           piSession,
		serviceInstanceID: serviceinstanceID,
	}, nil
}

func (s *powervsService) instanceClient(ctx context.Context) *instance.IBMPIInstanceClient {
	return instance.NewIBMPIInstanceClient(ctx, s.session, s.serviceInstanceID)
}

func (s *powervsService) dhcpClient(ctx context.Context) *instance.IBMPIDhcpClient {
	return instance.NewIBMPIDhcpClient(ctx, s.session, s.serviceInstanceID)
}

func newIdentityClient(auth core.Authenticator) (*iamidentityv1.IamIdentityV1, error) {
	identityv1Options := &iamidentityv1.IamIdentityV1Options{
		Authenticator: auth,
	}
	identityClient, err := iamidentityv1.NewIamIdentityV1(identityv1Options)
	if err != nil {
		return nil, err
	}
	return identityClient, nil
}

func getAccount(key string, identityClient *iamidentityv1.IamIdentityV1) (*string, error) {
	apikeyDetailsOptions := &iamidentityv1.GetAPIKeysDetailsOptions{
		IamAPIKey: &key,
	}

	apiKeyDetails, _, err := identityClient.GetAPIKeysDetails(apikeyDetailsOptions)
	if err != nil {
		return nil, err
	}
	return apiKeyDetails.AccountID, nil
}
