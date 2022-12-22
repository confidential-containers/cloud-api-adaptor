// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

type Factory interface {
	New(sandboxID string) AgentProxy
}

type factory struct {
	PauseImage    string
	CriSocketPath string
}

func NewFactory(pauseImage, criSocketPath string) Factory {
	return &factory{
		PauseImage:    pauseImage,
		CriSocketPath: criSocketPath,
	}
}

func (f *factory) New(socketPath string) AgentProxy {

	return NewAgentProxy(socketPath, f.CriSocketPath, f.PauseImage)
}
