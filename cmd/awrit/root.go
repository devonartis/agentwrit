// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// jsonOutput holds the value of the --json persistent flag. When true, commands
// print raw JSON instead of a formatted table.
var jsonOutput bool

// rootCmd is the top-level cobra command that all subcommands are attached to.
var rootCmd = &cobra.Command{
	Use:   "awrit",
	Short: "AgentAuth operator CLI",
	Long:  "awrit is the operator CLI for managing the AgentAuth broker.",
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output raw JSON")
}

// Execute runs the root cobra command and exits with status 1 on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
