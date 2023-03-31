// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/tlsutil"
)

type Factory interface {
	New(serverName, socketPath string) AgentProxy
}

type factory struct {
	pauseImage    string
	criSocketPath string
	tlsConfig     *tlsutil.TLSConfig
	caService     tlsutil.CAService
	proxyTimeout  time.Duration
}

func NewFactory(pauseImage, criSocketPath string, tlsConfig *tlsutil.TLSConfig, proxyTimeout time.Duration) Factory {

	if tlsConfig != nil && !tlsConfig.HasCertAuth() {

		certPEM, keyPEM, err := tlsutil.NewClientCertificate("cloud-api-adaptor")
		if err != nil {
			panic(err)
		}
		tlsConfig.CertData = certPEM
		tlsConfig.KeyData = keyPEM
	}

	var caService tlsutil.CAService

	if tlsConfig != nil && !tlsConfig.HasCA() {

		s, err := tlsutil.NewCAService("agent-protocol-forwarder")
		if err != nil {
			panic(err)
		}
		caService = s
		tlsConfig.CAData = caService.RootCertificate()
	}

	return &factory{
		pauseImage:    pauseImage,
		criSocketPath: criSocketPath,
		tlsConfig:     tlsConfig,
		caService:     caService,
		proxyTimeout:  proxyTimeout,
	}
}

func (f *factory) New(serverName, socketPath string) AgentProxy {

	return NewAgentProxy(serverName, socketPath, f.criSocketPath, f.pauseImage, f.tlsConfig, f.caService, f.proxyTimeout)
}
