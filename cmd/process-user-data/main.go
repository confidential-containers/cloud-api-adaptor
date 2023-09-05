// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	cmdUtil "github.com/confidential-containers/cloud-api-adaptor/cmd"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   programName,
	Short: "A program to process user data and update agent config file",
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag {
			cmdUtil.ShowVersion(programName) // nolint: errcheck
		} else if len(args) == 0 {
			cmd.Help() // nolint: errcheck
		}
	},
}

var versionFlag bool
var provisionFilesCmd = &cobra.Command{
	Use:          "provision-files",
	Short:        "Provision required files based on user data",
	RunE:         provisionFiles,
	SilenceUsage: true, // Silence usage on error
}

var updateAgentConfigCmd = &cobra.Command{
	Use:          "update-agent-config",
	Short:        "Update the agent configuration file",
	RunE:         updateAgentConfig,
	SilenceUsage: true, // Silence usage on error
}

var cfg Config

func init() {

	rootCmd.PersistentFlags().BoolVarP(&versionFlag, "version", "v", false, "Print the version")
	// Add a flag to specify the daemonConfigPath
	rootCmd.PersistentFlags().StringVarP(&cfg.daemonConfigPath, "daemon-config-path", "d", daemon.DefaultConfigPath, "Path to a daemon config file")

	// Add a flag to specify the timeout for fetching user data
	provisionFilesCmd.Flags().IntVarP(&cfg.userDataFetchTimeout, "user-data-fetch-timeout", "t", 180, "Timeout (in secs) for fetching user data")
	rootCmd.AddCommand(provisionFilesCmd)

	// Add a flag to specify the agentConfigPath to updateAgentConfigCmd subcommand
	updateAgentConfigCmd.Flags().StringVarP(&cfg.agentConfigPath, "agent-config-file", "a", defaultAgentConfigPath, "Path to a agent config file")

	rootCmd.AddCommand(updateAgentConfigCmd)

}

func main() {

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}

}
