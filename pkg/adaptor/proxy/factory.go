// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import "github.com/confidential-containers/cloud-api-adaptor/pkg/util/tls"

type Factory interface {
	New(sandboxID string) AgentProxy
}

type factory struct {
	PauseImage    string
	CriSocketPath string
	TLSConfig     *tls.TLSConfig
}

func NewFactory(pauseImage, criSocketPath string, tlsConfig *tls.TLSConfig) Factory {
	return &factory{
		PauseImage:    pauseImage,
		CriSocketPath: criSocketPath,
		TLSConfig:     tlsConfig,
	}
}

func (f *factory) New(socketPath string) AgentProxy {

	return NewAgentProxy(socketPath, f.CriSocketPath, f.PauseImage, f.TLSConfig)
}
