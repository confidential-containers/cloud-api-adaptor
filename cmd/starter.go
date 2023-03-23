// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/coreos/go-systemd/daemon"
	"golang.org/x/sys/unix"
)

type Starter interface {
	Start(context.Context) error
	List() []Service
}
type Service interface {
	Start(context.Context) error
	Ready() chan struct{}
}

type Config interface {
	Setup() (Starter, error)
}

type starter struct {
	services []*starterService
}
type starterService struct {
	Service
	errorCh chan error
	readyCh chan struct{}
}

func NewStarter(services ...Service) Starter {
	var s starter

	for _, service := range services {
		starterService := &starterService{
			Service: service,
			errorCh: make(chan error),
		}
		s.services = append(s.services, starterService)
	}
	return &s
}

func sdNotify(state string) {
	if _, err := daemon.SdNotify(false, state); err != nil {
		log.Printf("failed to send a notification to systemd: %v", err)
	}
}

func (s *starter) Start(ctx context.Context) error {

	ctx, cancel := context.WithCancel(ctx)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, unix.SIGTERM)

	go func() {
		defer close(sigCh)

		sig := <-sigCh

		log.Printf("Signal %s received. Shutting down", sig.String())

		cancel()

		sig = <-sigCh

		log.Printf("Signal %s received again. Force exiting", sig.String())

		Exit(1)
	}()

	for _, svc := range s.services {
		var service = svc

		go func() {
			defer close(service.errorCh)

			if err := service.Start(ctx); err != nil {
				cancel()
				service.errorCh <- err
			}
		}()

		service.readyCh = service.Ready()
	}

	if os.Getenv("NOTIFY_SOCKET") != "" {
		// Notify systemd when the services become ready using the sd_notify protocol
		// https://www.freedesktop.org/software/systemd/man/sd_notify.html#Description

		go func() {
			for _, service := range s.services {
				select {
				case <-ctx.Done():
					return
				case <-service.readyCh:
				}
			}

			sdNotify(daemon.SdNotifyReady)

			<-ctx.Done()

			sdNotify(daemon.SdNotifyStopping)
		}()
	}

	<-ctx.Done()

	if err := ctx.Err(); err != context.Canceled {
		return fmt.Errorf("context unexpectedly canceled: %w", err)
	}

	for _, starter := range s.services {
		if err := <-starter.errorCh; err != nil {
			return fmt.Errorf("error running a service %T: %w", starter.Service, err)
		}
	}

	return nil
}

func (s *starter) List() []Service {
	var list []Service
	for _, service := range s.services {
		list = append(list, service.Service)
	}
	return list
}
