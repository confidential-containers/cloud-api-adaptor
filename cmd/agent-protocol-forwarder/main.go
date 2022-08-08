// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/cmd"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder/interceptor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
)

const programName = "agent-protocol-forwarder"

type Config struct {
	configPath          string
	listenAddr          string
	kataAgentSocketPath string
	kataAgentNamespace  string
	HostInterface       string

	daemonConfig daemon.Config
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

	cmd.Parse(programName, os.Args, func(flags *flag.FlagSet) {
		flags.StringVar(&cfg.configPath, "config", daemon.DefaultConfigPath, "Path to a deamon config file")
		flags.StringVar(&cfg.listenAddr, "listen", daemon.DefaultListenAddr, "Listen address")
		flags.StringVar(&cfg.kataAgentSocketPath, "kata-agent-socket", daemon.DefaultKataAgentSocketPath, "Path to a kata agent socket")
		flags.StringVar(&cfg.kataAgentNamespace, "kata-agent-namespace", daemon.DefaultKataAgentNamespace, "Path to the network namespace where kata agent runs")
		flags.StringVar(&cfg.HostInterface, "host-interface", "", "network interface name that is used for network tunnel traffic")
	})

	for path, obj := range map[string]interface{}{
		cfg.configPath: &cfg.daemonConfig,
	} {
		if err := load(path, obj); err != nil {
			return nil, err
		}
	}

	interceptor := interceptor.NewInterceptor(cfg.kataAgentSocketPath, cfg.kataAgentNamespace)

	podNode := podnetwork.NewPodNode(cfg.kataAgentNamespace, cfg.HostInterface, cfg.daemonConfig.PodNetwork)

	daemon := daemon.NewDaemon(&cfg.daemonConfig, cfg.listenAddr, interceptor, podNode)

	return cmd.NewStarter(daemon), nil
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
