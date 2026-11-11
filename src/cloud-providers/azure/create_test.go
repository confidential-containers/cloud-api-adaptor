// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
)

// acceptedThenTransport accepts the VM PUT as an async operation and answers the operation poll with poll.
type acceptedThenTransport struct {
	poll func(*http.Request) (*http.Response, error)
}

func (t acceptedThenTransport) Do(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPut {
		resp := jsonResponse(req, http.StatusCreated, `{"properties":{"provisioningState":"Creating"}}`)
		resp.Header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/op1?api-version=2023-03-01")
		return resp, nil
	}
	return t.poll(req)
}

func jsonResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func TestCreateReturnsVMIDWhenPollFails(t *testing.T) {
	const wantID = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/podvm-test"

	tests := []struct {
		name string
		poll func(req *http.Request, cancel context.CancelFunc) (*http.Response, error)
	}{
		{
			name: "provisioning failed",
			poll: func(req *http.Request, _ context.CancelFunc) (*http.Response, error) {
				return jsonResponse(req, http.StatusOK, `{"status":"Failed","error":{"code":"Boom","message":"provisioning failed"}}`), nil
			},
		},
		{
			name: "context cancelled while polling",
			poll: func(req *http.Request, cancel context.CancelFunc) (*http.Response, error) {
				cancel()
				return nil, req.Context().Err()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			transport := acceptedThenTransport{poll: func(req *http.Request) (*http.Response, error) { return tt.poll(req, cancel) }}
			p := &azureProvider{
				azureClient:   &azfake.TokenCredential{},
				serviceConfig: &Config{SubscriptionID: "sub", ResourceGroupName: "rg"},
				clientOptions: &arm.ClientOptions{ClientOptions: azcore.ClientOptions{Transport: transport}},
			}
			params := &armcompute.VirtualMachine{
				Location: to.Ptr("eastus"),
				Properties: &armcompute.VirtualMachineProperties{
					OSProfile: &armcompute.OSProfile{ComputerName: to.Ptr("podvm-test")},
				},
			}

			vm, err := p.create(ctx, params)
			if err == nil {
				t.Fatal("expected an error")
			}
			if vm == nil || vm.ID == nil || *vm.ID != wantID {
				t.Fatalf("expected VM ID %q for cleanup, got %+v", wantID, vm)
			}
		})
	}
}
