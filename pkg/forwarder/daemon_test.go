// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"context"
	"net"
	"testing"
)

func TestNew(t *testing.T) {

	config := &Config{}

	ret := New(config, DefaultListenAddr, "dummy.sock", "", &mockPodNode{})
	if ret == nil {
		t.Fatal("Expect non nil, got nil")
	}
	d, ok := ret.(*daemon)
	if !ok {
		t.Fatalf("Expect *daemon, got %T", d)
	}
	if d.agentForwarder == nil {
		t.Fatal("Expect non nil, got nil")
	}
	if d.stopCh == nil {
		t.Fatal("Expect non nil, got nil")
	}
	select {
	case <-d.stopCh:
		t.Fatal("channel is closed")
	default:
	}
}

type mockForwarder struct{}

func (*mockForwarder) Start(ctx context.Context, listener net.Listener) error {
	<-ctx.Done()
	return nil
}

func (*mockForwarder) Shutdown() error {
	return nil
}

func TestStart(t *testing.T) {
	d := &daemon{
		agentForwarder: &mockForwarder{},
		podNode:        &mockPodNode{},
		readyCh:        make(chan struct{}),
		stopCh:         make(chan struct{}),
	}

	errCh := make(chan error)
	go func() {
		defer close(errCh)

		if err := d.Start(context.Background()); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		t.Fatalf("Expect no error, got %q", err)
	default:
	}

	if err := d.Shutdown(); err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Expect no error, got %q", err)
		}
	default:
	}
}

type mockPodNode struct{}

func (n *mockPodNode) Setup() error {
	return nil
}

func (n *mockPodNode) Teardown() error {
	return nil
}
