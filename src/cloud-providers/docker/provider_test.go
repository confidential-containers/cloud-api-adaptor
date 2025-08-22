// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"reflect"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/docker/docker/client"
)

func Test_dockerProvider_CreateInstance(t *testing.T) {
	type fields struct {
		Client *client.Client
	}

	testAPFConfigJSON := `{
		"pod-network": {
			"podip": "10.244.0.19/24",
			"pod-hw-addr": "0e:8f:62:f3:81:ad",
			"interface": "eth0",
			"worker-node-ip": "10.224.0.4/16",
			"tunnel-type": "vxlan",
			"routes": [
				{
					"Dst": "",
					"GW": "10.244.0.1",
					"Dev": "eth0"
				}
			],
			"mtu": 1500,
			"index": 1,
			"vxlan-port": 8472,
			"vxlan-id": 555001,
			"dedicated": false
		},
		"pod-namespace": "default",
		"pod-name": "nginx-866fdb5bfb-b98nw",
		"tls-server-key": "-----BEGIN PRIVATE KEY-----\n....\n-----END PRIVATE KEY-----\n",
		"tls-server-cert": "-----BEGIN CERTIFICATE-----\n....\n-----END CERTIFICATE-----\n",
		"tls-client-ca": "-----BEGIN CERTIFICATE-----\n....\n-----END CERTIFICATE-----\n"
	}`

	// Write tempAPFConfigJSON to cloud-init config file
	// Create a CloudConfig struct
	cloudConfig := &cloudinit.CloudConfig{
		WriteFiles: []cloudinit.WriteFile{
			{Path: "/run/peerpod/apf.json", Content: string(testAPFConfigJSON)},
		},
	}

	type args struct {
		ctx       context.Context
		podName   string
		sandboxID string
		spec      provider.InstanceTypeSpec
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *provider.Instance
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Test CreateInstance",
			fields: fields{
				Client: &client.Client{},
			},
			args: args{
				ctx:       context.Background(),
				podName:   "test",
				sandboxID: "test",
				spec:      provider.InstanceTypeSpec{},
			},
			want:    &provider.Instance{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &dockerProvider{
				Client: tt.fields.Client,
			}
			got, err := p.CreateInstance(tt.args.ctx, tt.args.podName, tt.args.sandboxID, cloudConfig, tt.args.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("dockerProvider.CreateInstance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dockerProvider.CreateInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}
