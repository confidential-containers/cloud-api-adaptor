// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM/go-sdk-core/v5/core"
)

// IBMCloudPowerVSAssert implements the CloudAssert interface for IBM Cloud PowerVS.
type IBMCloudPowerVSAssert struct {
	properties map[string]string
}

func newPowerVSInstanceClient(properties map[string]string) (*instance.IBMPIInstanceClient, error) {
	options := &ibmpisession.IBMPIOptions{
		Authenticator: &core.IamAuthenticator{
			ApiKey: properties["IBMCLOUD_API_KEY"],
		},
		UserAccount: properties["IBMCLOUD_ACCOUNT_ID"],
		Zone:        properties["POWERVS_ZONE"],
	}

	piSession, err := ibmpisession.NewIBMPISession(options)
	if err != nil {
		return nil, err
	}

	return instance.NewIBMPIInstanceClient(context.Background(), piSession, properties["POWERVS_SERVICE_INSTANCE_ID"]), nil
}

// DefaultTimeout returns the default timeout used by shared e2e tests for
// PowerVS cloud operations, such as waiting for PodVM deletion.
func (c IBMCloudPowerVSAssert) DefaultTimeout() time.Duration {
	return 30 * time.Second
}

func (c IBMCloudPowerVSAssert) HasPodVM(t *testing.T, podvmName string) {
	instanceClient, err := newPowerVSInstanceClient(c.properties)
	if err != nil {
		t.Fatalf("Podvm name=%s not found: %v", podvmName, err)
	}

	instances, err := instanceClient.GetAll()
	if err != nil {
		t.Fatalf("Podvm name=%s not found: %v", podvmName, err)
	}

	// Validate that the backing PowerVS VM for the pod was created. The expected
	// PodVM name is derived from CAA logs and should match the instance ServerName.
	found := false

	for _, instance := range instances.PvmInstances {
		if instance == nil || instance.ServerName == nil {
			continue
		}
		if *instance.ServerName == podvmName {
			found = true
		}
	}

	if !found {
		t.Fatalf("Podvm name=%s not found", podvmName)
	}
}

func (c IBMCloudPowerVSAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	return "", nil
}
