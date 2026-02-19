// Command aactl revoke subcommand — revokes tokens at various granularity levels.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// revokeLevel holds the --level flag value for the revoke command.
var revokeLevel string

// revokeTarget holds the --target flag value for the revoke command.
var revokeTarget string

// revokeCmd revokes broker tokens at a specified granularity level.
// Supported levels: token (by JTI), agent (by SPIFFE ID), task (by task ID),
// chain (all tokens in a delegation chain rooted at the given agent).
var revokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke tokens (token, agent, task, or chain level)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if revokeLevel == "" || revokeTarget == "" {
			return fmt.Errorf("--level and --target are required")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		payload := map[string]string{"level": revokeLevel, "target": revokeTarget}
		data, err := c.doPost("/v1/revoke", payload)
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			Revoked bool   `json:"revoked"`
			Level   string `json:"level"`
			Target  string `json:"target"`
			Count   int    `json:"count"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		printTable(
			[]string{"REVOKED", "LEVEL", "TARGET", "COUNT"},
			[][]string{{
				fmt.Sprintf("%v", resp.Revoked),
				resp.Level,
				resp.Target,
				fmt.Sprintf("%d", resp.Count),
			}},
		)
		return nil
	},
}

func init() {
	revokeCmd.Flags().StringVar(&revokeLevel, "level", "", "revocation level: token|agent|task|chain (required)")
	revokeCmd.Flags().StringVar(&revokeTarget, "target", "", "revocation target (required)")
	rootCmd.AddCommand(revokeCmd)
}
