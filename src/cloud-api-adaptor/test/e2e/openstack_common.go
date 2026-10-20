// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/openstack"
	"github.com/gophercloud/gophercloud/v2"
	gophcos "github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/pagination"
)

// OpenStackCloudAssert implements the CloudAssert interface for openstack.
type OpenStackCloudAssert struct{}

// findOpenStackVM searches for a VM by its name in OpenStack and returns the server object if found.
func findOpenStackVM(podvmName string) (*servers.Server, error) {
	client, err := gophcos.NewComputeV2(
		pv.OpenStackProvs.OpenStackClient,
		gophercloud.EndpointOpts{
			Region: pv.OpenStackProvs.Properties["OPENSTACK_REGION"],
		})
	if err != nil {
		return nil, fmt.Errorf("unable to create openstack compute client: %v", err)
	}

	pager := servers.List(client, nil)
	var serverInfo *servers.Server = nil
	err = pager.EachPage(context.Background(), func(ctx context.Context, page pagination.Page) (bool, error) {
		serverList, err := servers.ExtractServers(page)
		if err != nil {
			return false, err
		}
		for _, s := range serverList {
			if s.Name == podvmName {
				serverInfo = &s
				return false, nil
			}
		}
		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error listing servers: %v", err)
	}
	return serverInfo, nil
}

// NewOpenStackAssert creates a new instance of OpenStackCloudAssert.
func NewOpenStackAssert() OpenStackCloudAssert {
	return OpenStackCloudAssert{}
}

// DefaultTimeout returns the default timeout duration for OpenStack operations.
func (c OpenStackCloudAssert) DefaultTimeout() time.Duration {
	return 2 * time.Minute
}

// HasPodVM checks if a PodVM with the given name exists in OpenStack.
func (c OpenStackCloudAssert) HasPodVM(t *testing.T, podvmName string) {
	server, err := findOpenStackVM(podvmName)
	if server != nil {
		t.Logf("Vitural machine %s found.", podvmName)
	} else {
		t.Logf("Virtual machine %s not found: %s", podvmName, err)
		t.Error("PodVM was not created")
	}
}

// GetInstanceType retrieves the instance type (ID) of the PodVM associated with the given pod name.
func (c OpenStackCloudAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	prefixName := "podvm-" + podName
	server, err := findOpenStackVM(prefixName)
	if err != nil || server == nil {
		t.Logf("Virtual machine %s not found: %s", podName, err)
		return "", err
	}

	return server.ID, nil
}
