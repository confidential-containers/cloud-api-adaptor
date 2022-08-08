// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/agentproto"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
)

var logger = log.New(log.Writer(), "[forwarder/interceptor] ", log.LstdFlags|log.Lmsgprefix)

type Interceptor interface {
	agentproto.Redirector
}

func NewInterceptor(agentSocket, nsPath string) Interceptor {

	agentDialer := func(ctx context.Context) (net.Conn, error) {

		if nsPath == "" {
			return (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
		}

		ns, err := netops.NewNSFromPath(nsPath)
		if err != nil {
			err = fmt.Errorf("failed to open network namespace %q: %w", nsPath, err)
			logger.Print(err)
			return nil, err
		}

		var conn net.Conn
		if err := ns.Run(func() error {
			var err error
			conn, err = (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
			return err
		}); err != nil {
			return nil, fmt.Errorf("failed to call dialer at namespace %q: %w", nsPath, err)
		}

		return conn, nil
	}

	interceptor := agentproto.NewRedirector(agentDialer)

	return interceptor
}
