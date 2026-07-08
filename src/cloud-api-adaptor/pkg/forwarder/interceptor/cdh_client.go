// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/containerd/ttrpc"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder/interceptor/cdhpb"
)

const cdhServiceName = "api.SecureMountService"

type cdhClient struct {
	conn   net.Conn
	client *ttrpc.Client
}

func newCDHClient(socketPath string) (*cdhClient, error) {
	const maxAttempts = 10
	const retryDelay = 2 * time.Second

	var conn net.Conn
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err = net.DialTimeout("unix", socketPath, 5*time.Second)
		if err == nil {
			break
		}
		if attempt < maxAttempts {
			logger.Printf("CDH socket %s not ready (attempt %d/%d): %v", socketPath, attempt, maxAttempts, err)
			time.Sleep(retryDelay)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("dialing CDH socket %s after %d attempts: %w", socketPath, maxAttempts, err)
	}

	client := ttrpc.NewClient(conn)
	return &cdhClient{conn: conn, client: client}, nil
}

func (c *cdhClient) close() {
	if err := c.client.Close(); err != nil {
		logger.Printf("WARNING: closing CDH ttrpc client: %v", err)
	}
	if err := c.conn.Close(); err != nil {
		logger.Printf("WARNING: closing CDH connection: %v", err)
	}
}

func (c *cdhClient) secureMount(ctx context.Context, volumeType string, options map[string]string, flags []string, mountPoint string) (string, error) {
	req := &cdhpb.SecureMountRequest{
		VolumeType: volumeType,
		Options:    options,
		Flags:      flags,
		MountPoint: mountPoint,
	}
	resp := &cdhpb.SecureMountResponse{}

	if err := c.client.Call(ctx, cdhServiceName, "SecureMount", req, resp); err != nil {
		return "", fmt.Errorf("CDH SecureMount RPC failed: %w", err)
	}

	return resp.GetMountPath(), nil
}
