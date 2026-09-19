// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloudpowervs

import (
	"context"

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM/go-sdk-core/v5/core"
)

type powervsService struct {
	session           *ibmpisession.IBMPISession
	serviceInstanceID string
}

func newPowervsClient(apikey, accountID, serviceinstanceID, zone string) (*powervsService, error) {
	options := &ibmpisession.IBMPIOptions{}
	options.Authenticator = &core.IamAuthenticator{
		ApiKey: apikey,
	}
	options.UserAccount = accountID
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
