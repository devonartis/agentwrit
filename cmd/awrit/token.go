// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

// Command awrit token subcommand — token operations including release.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// releaseToken holds the --token flag value for the token release command.
var releaseToken string

// tokenCmd is the parent command for token operations.
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Token operations (release)",
}

// tokenReleaseCmd calls POST /v1/token/release using the provided Bearer token.
// This is an agent-facing endpoint — the token being presented is the token
// being released (self-revocation). Operators use this to test the release flow
// or to force-release a token they possess.
var tokenReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release (self-revoke) an agent token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if releaseToken == "" {
			return fmt.Errorf("--token is required")
		}
		url := os.Getenv("AACTL_BROKER_URL")
		if url == "" {
			return fmt.Errorf("AACTL_BROKER_URL is not set")
		}
		c := &client{baseURL: url, http: defaultHTTPClient()}
		status, body, err := c.doPostWithToken("/v1/token/release", releaseToken)
		if err != nil {
			return err
		}
		if status == 204 {
			fmt.Println("Token released successfully.")
			return nil
		}
		// A 403 "token has been revoked" means the token was already released —
		// the middleware rejects it before the handler runs. This is idempotent
		// from the operator's perspective.
		if status == 403 && strings.Contains(string(body), "revoked") {
			fmt.Println("Token already released (revoked).")
			return nil
		}
		if status >= 400 {
			return fmt.Errorf("HTTP %d: %s", status, string(body))
		}
		fmt.Printf("HTTP %d: %s\n", status, string(body))
		return nil
	},
}

func init() {
	tokenReleaseCmd.Flags().StringVar(&releaseToken, "token", "", "the agent JWT to release (required)")
	tokenCmd.AddCommand(tokenReleaseCmd)
	rootCmd.AddCommand(tokenCmd)
}
