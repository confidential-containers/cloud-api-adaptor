// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type mockService struct {
	once    sync.Once
	readyCh chan struct{}
	stopCh  chan struct{}
	error   error
}

func (m *mockService) Start(ctx context.Context) error {

	if m.error != nil {
		return m.error
	}

	if m.stopCh != nil {
		<-m.stopCh
		log.Printf("Killed")
		return nil
	}

	close(m.Ready())

	<-ctx.Done()

	if err := ctx.Err(); err != context.Canceled {
		return errors.New("service canceled")
	}
	log.Printf("Shut down")

	return nil
}

func (m *mockService) Ready() chan struct{} {
	m.once.Do(func() {
		m.readyCh = make(chan struct{})
	})
	return m.readyCh
}

func run(errCh chan error, exitCh chan struct{}, fn func() error) string {
	defer close(errCh)

	if exitCh != nil {
		Exit = func(_ int) {
			close(exitCh)
		}
	}

	old := log.Writer()
	defer func() {
		log.SetOutput(old)
	}()

	var buffer bytes.Buffer
	log.SetOutput(&buffer)
	log.SetFlags(0)

	err := fn()

	if err != nil {
		errCh <- err
	}

	return buffer.String()
}

func kill(t *testing.T) {
	t.Helper()

	if err := unix.Kill(unix.Getpid(), unix.SIGINT); err != nil {
		t.Fatalf("Expect no error, got %#v", err)
	}
}

func TestStarter(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())

	exitCh := make(chan struct{})
	starter := NewStarter(&mockService{}, &mockService{stopCh: exitCh})
	if starter == nil {
		t.Fatalf("Expect non nil, got %v", starter)
	}

	errCh := make(chan error, 1)
	var output string

	go func() {
		defer cancel()

		output = run(errCh, exitCh, func() error {
			return starter.Start(ctx)
		})
	}()

	time.Sleep(time.Second)

	kill(t)

	time.Sleep(time.Second)

	kill(t)

	<-ctx.Done()

	if err := <-errCh; err != nil {
		t.Fatalf("Expect no error, got %#v", err)
	}
	msg := "Signal interrupt received. Shutting down\nShut down\nSignal interrupt received again. Force exiting\nKilled\n"
	if e, a := msg, output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
}

func prefix(errStr string) string {
	i := strings.Index(errStr, ":")
	return errStr[:i]
}

func TestStarterWithError(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())

	serviceError := errors.New("failed to start")
	starter := NewStarter(&mockService{}, &mockService{error: serviceError})
	if starter == nil {
		t.Fatalf("Expect non nil, got %v", starter)
	}

	exitCh := make(chan struct{})
	errCh := make(chan error, 1)
	var output string

	go func() {
		defer cancel()

		output = run(errCh, exitCh, func() error {
			return starter.Start(ctx)
		})
	}()

	<-ctx.Done()
	err := <-errCh

	if e, a := "error running a service *cmd.mockService", prefix(err.Error()); e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := serviceError, errors.Unwrap(err); e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
	if e, a := "Shut down\n", output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
}

func TestStarterWithTimeout(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	starter := NewStarter(&mockService{})
	if starter == nil {
		t.Fatalf("Expect non nil, got %v", starter)
	}

	exitCh := make(chan struct{})
	errCh := make(chan error, 1)
	var output string

	go func() {
		defer cancel()

		output = run(errCh, exitCh, func() error {
			return starter.Start(ctx)
		})
	}()

	<-ctx.Done()
	err := <-errCh

	if e, a := "context unexpectedly canceled", prefix(err.Error()); e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := context.DeadlineExceeded, errors.Unwrap(err); e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
	if e, a := "", output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
}

func TestStarterList(t *testing.T) {

	for _, services := range [][]Service{
		{},
		{&mockService{}},
		{&mockService{}, &mockService{}},
	} {
		starter := NewStarter(services...)
		if starter == nil {
			t.Fatalf("Expect non nil, got %v", starter)
		}
		list := starter.List()
		if e, a := len(services), len(list); e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}
		for i := range services {
			if e, a := services[i], list[i]; e != a {
				t.Fatalf("Expect %v, got %v", e, a)
			}
		}
	}
}

func TestStarterServiceReady(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	service := &mockService{}

	starter := NewStarter(service)

	errCh := make(chan error, 1)
	var output string

	select {
	case <-service.Ready():
		t.Fatalf("becomes ready before started")
	default:
	}

	go func() {
		output = run(errCh, nil, func() error {
			return starter.Start(ctx)
		})
	}()

	select {
	case <-ctx.Done():
		t.Fatalf("completed before becoming ready")
	case <-service.Ready():
	}

	cancel()

	<-ctx.Done()

	if err := <-errCh; err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
	expected := "Shut down\n"
	if expected != output {
		t.Fatalf("Expect %q, got %q", expected, output)
	}
}
