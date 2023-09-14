package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	toml "github.com/pelletier/go-toml/v2"
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

	// Parse the agent config file
	agentConfig, err := parseAgentConfig(cfg.agentConfigPath)
	if err != nil {
		return fmt.Errorf("failed to parse agent config file: %s", err)
	}

	if config.AAKBCParams != "" {
		fmt.Printf("Updating aa_kbc_params in agent config file")
		agentConfig.AaKbcParams = config.AAKBCParams
	}

	if config.AuthJson != "" {

		fmt.Printf("Updating image_registry_auth_file in agent config file with value\n")

		// Check if authJsonFilePath exists. If it doesn't exists create the file

		if _, err := os.Stat(defaultAuthJsonFilePath); err != nil && os.IsNotExist(err) {
			// Write the authJson to the defaultAuthJsonFilePath
			err = os.WriteFile(defaultAuthJsonFilePath, []byte(config.AuthJson), 0644)
			if err != nil {
				return fmt.Errorf("failed to write auth.json file: %s", err)
			}
		}

		// Update the file path in the agent config
		agentConfig.ImageRegistryAuthFile = "file://" + defaultAuthJsonFilePath

	}

	// Write the updated agent config file
	err = writeAgentConfig(*agentConfig, cfg.agentConfigPath)
	if err != nil {
		return fmt.Errorf("failed to write agent config file: %s", err)
	}

	return nil
}

// Kata agent config is a TOML file, parse it and return the AgentConfig struct
func parseAgentConfig(agentConfigFile string) (agentConfig *AgentConfig, err error) {

	agentConfig = &AgentConfig{}

	data, err := os.ReadFile(agentConfigFile)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil, err
	}

	// Parse the agent config file data
	err = toml.Unmarshal(data, agentConfig)
	if err != nil {
		fmt.Println("Error parsing agent config file:", err)
		return nil, err
	}

	return agentConfig, nil
}

// Write the agent config file
func writeAgentConfig(agentConfig AgentConfig, agentConfigFile string) error {

	data, err := toml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("error marshalling agent config: %s", err)
	}

	// Write the newAgentConfig to the agentConfigFile
	err = os.WriteFile(agentConfigFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write agent config file: %s", err)
	}

	fmt.Printf("Updated agent config file: %s\n", agentConfigFile)
	return nil
}
