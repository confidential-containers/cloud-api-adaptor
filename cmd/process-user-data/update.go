package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/spf13/cobra"
)

// Get daemon.Config from local file
func getConfigFromLocalFile(daemonConfigPath string) daemon.Config {

	// if daemonConfigPath is empty then return
	if daemonConfigPath == "" {
		fmt.Printf("daemonConfigPath is empty\n")
		return daemon.Config{}
	}

	// Read the daemonConfigPath file
	daemonConfig, err := os.ReadFile(daemonConfigPath)
	if err != nil {
		fmt.Printf("failed to read daemon config file: %s\n", err)
		return daemon.Config{}
	}

	// UnMarshal the daemonConfig into forwarder (daemon) Config struct
	var config daemon.Config

	err = json.Unmarshal(daemonConfig, &config)
	if err != nil {
		fmt.Printf("failed to unmarshal daemon config: %s\n", err)
		return daemon.Config{}
	}

	return config
}

// Add method to get the value of aa-kbc-param from userdata and replace the value of aa_kbc_params in the
// /etc/agent-config.toml file
func updateAAKBCParams(aaKBCParams string, agentConfigFile string) error {

	// if aaKBCParams is empty then return. Nothing to do
	if aaKBCParams == "" {
		fmt.Printf("aaKBCParams is empty. Nothing to do\n")
		return nil
	}

	if agentConfigFile == "" {
		return fmt.Errorf("agentConfigFile is empty")
	}

	// Replace the aa_kbc_params line in agentConfigFile with the aaKBCParams value
	// Read the agentConfigFile
	agentConfig, err := os.ReadFile(agentConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read agent config file: %s", err)
	}

	// Split the agentConfigFile into lines
	lines := strings.Split(string(agentConfig), "\n")

	// Loop through the lines and replace the line that starts with aa_kbc_params
	for i, line := range lines {
		if strings.Contains(line, "aa_kbc_params") {
			lines[i] = fmt.Sprintf("aa_kbc_params = \"%s\"", aaKBCParams)
			fmt.Printf("Updated line: %s\n", lines[i])
		}
	}

	// Join the lines back into a string
	newAgentConfig := strings.Join(lines, "\n")

	// Write the newAgentConfig to the agentConfigFile
	err = os.WriteFile(agentConfigFile, []byte(newAgentConfig), 0644)
	if err != nil {
		return fmt.Errorf("failed to write agent config file: %s", err)
	}

	fmt.Printf("Updated agent config file: %s\n", agentConfigFile)

	return nil
}

func updateAgentConfig(cmd *cobra.Command, args []string) error {

	// Get the daemon.Config from the daemonConfigPath
	// It's assumed that the local file is already provisioned either via the provision-files command
	// or via some other means
	config := getConfigFromLocalFile(cfg.daemonConfigPath)
	if config == (daemon.Config{}) {
		return fmt.Errorf("failed to get daemon config from local file")
	}

	// Replace the value of aa_kbc_params in the agent config file - default: /etc/agent-config.toml
	// with the value of aa-kbc-params
	if err := updateAAKBCParams(config.AAKBCParams, cfg.agentConfigPath); err != nil {
		fmt.Printf("Error: Failed to update agent config file with aa-kbc-params: %s\n", err)
		return err
	}

	// TODO: Add code to update the agent config file with the values of the other fields in daemon.Config

	return nil
}
