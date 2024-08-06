// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	cmdUtil "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/cmd"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/userdata"
	"github.com/spf13/cobra"
)

const (
	programName   = "process-user-data"
	providerAzure = "azure"
	providerAws   = "aws"
)

var versionFlag bool
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

func init() {
	var fetchTimeout int
	rootCmd.PersistentFlags().BoolVarP(&versionFlag, "version", "v", false, "Print the version")

	var provisionFilesCmd = &cobra.Command{
		Use:   "provision-files",
		Short: "Provision required files based on user data",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := userdata.NewConfig(fetchTimeout)
			return userdata.ProvisionFiles(cfg)
		},
		SilenceUsage: true, // Silence usage on error
	}
	provisionFilesCmd.Flags().IntVarP(&fetchTimeout, "user-data-fetch-timeout", "t", 180, "Timeout (in secs) for fetching user data")
	rootCmd.AddCommand(provisionFilesCmd)
}

func main() {

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}

}
