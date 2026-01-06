// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/cmd"
	daemon "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder/interceptor"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
)

const (
	programName          = "agent-protocol-forwarder"
	API_SERVER_REST_PORT = 8006
)

type Config struct {
	tlsConfig           *tlsutil.TLSConfig
	daemonConfig        daemon.Config
	configPath          string
	listenAddr          string
	kataAgentSocketPath string
	podNamespace        string
	HostInterface       string
}

func load(path string, obj interface{}) error {

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}

	if err := json.NewDecoder(file).Decode(obj); err != nil {
		return fmt.Errorf("failed to decode a Agent Protocol Forwarder config file file: %s: %w", path, err)
	}

	return nil
}

func (cfg *Config) Setup() (cmd.Starter, error) {
	var (
		showVersion bool
		disableTLS  bool
		tlsConfig   tlsutil.TLSConfig
		services    []cmd.Service
	)

	cmd.Parse(programName, os.Args, func(flags *flag.FlagSet) {
		flags.BoolVar(&showVersion, "version", false, "Show version")
		flags.StringVar(&cfg.configPath, "config", daemon.DefaultConfigPath, "Path to a daemon config file")
		flags.StringVar(&cfg.listenAddr, "listen", daemon.DefaultListenAddr, "Listen address")
		flags.StringVar(&cfg.kataAgentSocketPath, "kata-agent-socket", daemon.DefaultKataAgentSocketPath, "Path to a kata agent socket")
		flags.StringVar(&cfg.podNamespace, "pod-namespace", daemon.DefaultPodNamespace, "Path to the network namespace where the pod runs")
		flags.StringVar(&cfg.HostInterface, "host-interface", "", "network interface name that is used for network tunnel traffic")
		flags.StringVar(&tlsConfig.CAFile, "ca-cert-file", "", "CA cert file")
		flags.StringVar(&tlsConfig.CertFile, "cert-file", "", "cert file")
		flags.StringVar(&tlsConfig.KeyFile, "cert-key", "", "cert key")
		flags.BoolVar(&tlsConfig.SkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification - use it only for testing")
		flags.BoolVar(&disableTLS, "disable-tls", false, "Disable TLS encryption - use it only for testing")
	})

	cmd.ShowVersion(programName)

	if showVersion {
		cmd.Exit(0)
	}

	if err := load(cfg.configPath, &cfg.daemonConfig); err != nil {
		return nil, err
	}

	if !disableTLS {
		cfg.tlsConfig = &tlsConfig
	}

	interceptor := interceptor.NewInterceptor(cfg.kataAgentSocketPath, cfg.podNamespace)

	podNode := podnetwork.NewPodNode(cfg.podNamespace, cfg.HostInterface, cfg.daemonConfig.PodNetwork)

	services = append(services, daemon.NewDaemon(&cfg.daemonConfig, cfg.listenAddr, cfg.tlsConfig, interceptor, podNode))

	return cmd.NewStarter(services...), nil
}

var config cmd.Config = &Config{}

func main() {

	starter, err := config.Setup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		cmd.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := starter.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		cmd.Exit(1)
	}
}
