// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

type Starter interface {
	Start(context.Context) error
	List() []Service
}
type Service interface {
	Start(context.Context) error
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
