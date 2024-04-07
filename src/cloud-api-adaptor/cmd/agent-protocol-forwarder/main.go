// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/cmd"
	daemon "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder/interceptor"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/apic"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/ppssh"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
)

const (
	programName          = "agent-protocol-forwarder"
	API_SERVER_REST_PORT = 8006
)

var logger = log.New(log.Writer(), "[forwarder] ", log.LstdFlags|log.Lmsgprefix)

type Config struct {
	tlsConfig           *tlsutil.TLSConfig
	daemonConfig        daemon.Config
	configPath          string
	listenAddr          string
	kataAgentSocketPath string
	kataAgentNamespace  string
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
		showVersion          bool
		disableTLS           bool
		secureComms          bool
		secureCommsInbounds  string
		secureCommsOutbounds string
		tlsConfig            tlsutil.TLSConfig
		services             []cmd.Service
	)

	cmd.Parse(programName, os.Args, func(flags *flag.FlagSet) {
		flags.BoolVar(&showVersion, "version", false, "Show version")
		flags.StringVar(&cfg.configPath, "config", daemon.DefaultConfigPath, "Path to a daemon config file")
		flags.StringVar(&cfg.listenAddr, "listen", daemon.DefaultListenAddr, "Listen address")
		flags.StringVar(&cfg.kataAgentSocketPath, "kata-agent-socket", daemon.DefaultKataAgentSocketPath, "Path to a kata agent socket")
		flags.StringVar(&cfg.kataAgentNamespace, "kata-agent-namespace", daemon.DefaultKataAgentNamespace, "Path to the network namespace where kata agent runs")
		flags.StringVar(&cfg.HostInterface, "host-interface", "", "network interface name that is used for network tunnel traffic")
		flags.StringVar(&tlsConfig.CAFile, "ca-cert-file", "", "CA cert file")
		flags.StringVar(&tlsConfig.CertFile, "cert-file", "", "cert file")
		flags.StringVar(&tlsConfig.KeyFile, "cert-key", "", "cert key")
		flags.BoolVar(&tlsConfig.SkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification - use it only for testing")
		flags.BoolVar(&disableTLS, "disable-tls", false, "Disable TLS encryption - use it only for testing")
		flags.BoolVar(&secureComms, "secure-comms", false, "Use SSH to secure communication between cluster and peer pods")
		flags.StringVar(&secureCommsInbounds, "secure-comms-inbounds", "", "Inbound tags for secure communication tunnels")
		flags.StringVar(&secureCommsOutbounds, "secure-comms-outbounds", "", "Outbound tags for secure communication tunnels")
	})

	cmd.ShowVersion(programName)

	if showVersion {
		cmd.Exit(0)
	}

	if err := load(cfg.configPath, &cfg.daemonConfig); err != nil {
		return nil, err
	}

	if secureComms {
		ppssh.Singleton()
		host, port, err := net.SplitHostPort(cfg.listenAddr)
		if err != nil {
			logger.Printf("Warning: illegal address %s is changed to 127.0.0.1:%s since secure-comms is enabled.", cfg.listenAddr, port)
		}
		if host != "127.0.0.1" {
			logger.Printf("Address %s is changed to 127.0.0.1:%s since secure-comms is enabled.", cfg.listenAddr, port)
			cfg.listenAddr = "127.0.0.1:" + port
		}
		inbounds := append([]string{"BOTH_PHASES:KBS:8080"}, strings.Split(secureCommsInbounds, ",")...)
		outbounds := append([]string{"KUBERNETES_PHASE:KATAAGENT:" + port}, strings.Split(secureCommsOutbounds, ",")...)

		// Create a Client that will approach the api-server-rest service at the podns
		// To obtain secrets from KBS, we approach the api-server-rest service which then approaches the CDH asking for a secret resource
		// the CDH than contact the KBS (possibly after approaching Attestation Agent for a token) and the KBS serves the requested key
		// The communication between the CDH (and Attestation Agent) and the KBS is performed via an SSH tunnel named  "KBS"
		apic := apic.NewApiClient(API_SERVER_REST_PORT, cfg.kataAgentNamespace)
		services = append(services, ppssh.NewSshServer(inbounds, outbounds, ppssh.GetSecret(apic.GetKey), sshutil.SSHPORT))
	} else {
		if !disableTLS {
			cfg.tlsConfig = &tlsConfig
		}
	}

	interceptor := interceptor.NewInterceptor(cfg.kataAgentSocketPath, cfg.kataAgentNamespace)

	podNode := podnetwork.NewPodNode(cfg.kataAgentNamespace, cfg.HostInterface, cfg.daemonConfig.PodNetwork)

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
