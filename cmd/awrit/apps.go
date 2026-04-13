// SPDX-License-Identifier: PolyForm-Internal-Use-1.0.0

// Package main — awrit app subcommands for managing registered apps.
//
// Commands:
//
//	awrit app register --name NAME --scopes SCOPE_CSV [--token-ttl N]
//	awrit app list [--json]
//	awrit app get APP_ID
//	awrit app update --id APP_ID [--scopes SCOPE_CSV] [--token-ttl N]
//	awrit app remove --id APP_ID
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// appCmd is the parent command grouping all app-related subcommands
// under "awrit app".
var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage registered apps",
}

// appRegisterName, appRegisterScopes, and appRegisterTokenTTL hold flag values
// for "awrit app register".
var (
	appRegisterName     string
	appRegisterScopes   string
	appRegisterTokenTTL int
)

// appRegisterCmd implements "awrit app register", creating a new app registration
// and printing the generated client_id and client_secret.
var appRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new app and receive credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if appRegisterName == "" {
			return fmt.Errorf("--name is required")
		}
		if appRegisterScopes == "" {
			return fmt.Errorf("--scopes is required")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		scopes := strings.Split(appRegisterScopes, ",")
		payload := map[string]any{
			"name":   appRegisterName,
			"scopes": scopes,
		}
		if cmd.Flags().Changed("token-ttl") {
			payload["token_ttl"] = appRegisterTokenTTL
		}
		data, err := c.doPost("/v1/admin/apps", payload)
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			AppID        string   `json:"app_id"`
			ClientID     string   `json:"client_id"`
			ClientSecret string   `json:"client_secret"`
			Scopes       []string `json:"scopes"`
			TokenTTL     int      `json:"token_ttl"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		printTable(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"APP_ID", resp.AppID},
				{"CLIENT_ID", resp.ClientID},
				{"CLIENT_SECRET", resp.ClientSecret},
				{"SCOPES", strings.Join(resp.Scopes, ", ")},
				{"TOKEN_TTL", fmt.Sprintf("%ds", resp.TokenTTL)},
			},
		)
		fmt.Fprintln(os.Stderr, "\nWARNING: Save the client_secret — it cannot be retrieved again.")
		return nil
	},
}

// appListCmd implements "awrit app list", printing all registered apps.
var appListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		data, err := c.doGet("/v1/admin/apps")
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			Apps []struct {
				AppID     string   `json:"app_id"`
				Name      string   `json:"name"`
				ClientID  string   `json:"client_id"`
				Scopes    []string `json:"scopes"`
				TokenTTL  int      `json:"token_ttl"`
				Status    string   `json:"status"`
				CreatedAt string   `json:"created_at"`
			} `json:"apps"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		rows := make([][]string, len(resp.Apps))
		for i, a := range resp.Apps {
			rows[i] = []string{
				a.Name,
				a.AppID,
				a.ClientID,
				a.Status,
				strings.Join(a.Scopes, ","),
				fmt.Sprintf("%ds", a.TokenTTL),
				a.CreatedAt,
			}
		}
		printTable([]string{"NAME", "APP_ID", "CLIENT_ID", "STATUS", "SCOPES", "TOKEN_TTL", "CREATED"}, rows)
		fmt.Fprintf(os.Stderr, "Total: %d\n", resp.Total)
		return nil
	},
}

// appGetCmd implements "awrit app get APP_ID", printing full details for one app.
var appGetCmd = &cobra.Command{
	Use:   "get APP_ID",
	Short: "Get details of a specific app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		data, err := c.doGet("/v1/admin/apps/" + args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			AppID     string   `json:"app_id"`
			Name      string   `json:"name"`
			ClientID  string   `json:"client_id"`
			Scopes    []string `json:"scopes"`
			TokenTTL  int      `json:"token_ttl"`
			Status    string   `json:"status"`
			CreatedAt string   `json:"created_at"`
			UpdatedAt string   `json:"updated_at"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		printTable(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"APP_ID", resp.AppID},
				{"NAME", resp.Name},
				{"CLIENT_ID", resp.ClientID},
				{"STATUS", resp.Status},
				{"SCOPES", strings.Join(resp.Scopes, ", ")},
				{"TOKEN_TTL", fmt.Sprintf("%ds", resp.TokenTTL)},
				{"CREATED", resp.CreatedAt},
				{"UPDATED", resp.UpdatedAt},
			},
		)
		return nil
	},
}

// appUpdateID, appUpdateScopes, and appUpdateTokenTTL hold flag values
// for "awrit app update".
var (
	appUpdateID       string
	appUpdateScopes   string
	appUpdateTokenTTL int
)

// appUpdateCmd implements "awrit app update --id APP_ID [--scopes SCOPE_CSV] [--token-ttl N]",
// updating scope ceiling and/or token TTL for an existing app.
var appUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an app's settings (scopes and/or token TTL)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if appUpdateID == "" {
			return fmt.Errorf("--id is required")
		}
		if appUpdateScopes == "" && !cmd.Flags().Changed("token-ttl") {
			return fmt.Errorf("at least one of --scopes or --token-ttl is required")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		payload := map[string]any{}
		if appUpdateScopes != "" {
			payload["scopes"] = strings.Split(appUpdateScopes, ",")
		}
		if cmd.Flags().Changed("token-ttl") {
			payload["token_ttl"] = appUpdateTokenTTL
		}
		data, err := c.doPut("/v1/admin/apps/"+appUpdateID, payload)
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			AppID     string   `json:"app_id"`
			Scopes    []string `json:"scopes"`
			TokenTTL  int      `json:"token_ttl"`
			UpdatedAt string   `json:"updated_at"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		printTable(
			[]string{"APP_ID", "SCOPES", "TOKEN_TTL", "UPDATED_AT"},
			[][]string{{resp.AppID, strings.Join(resp.Scopes, ", "), fmt.Sprintf("%ds", resp.TokenTTL), resp.UpdatedAt}},
		)
		return nil
	},
}

// appRemoveID holds the --id flag value for "awrit app remove".
var appRemoveID string

// appRemoveCmd implements "awrit app remove --id APP_ID", deregistering an app
// (soft delete — credentials stop working but record is retained).
var appRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Deregister an app (credentials stop working immediately)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if appRemoveID == "" {
			return fmt.Errorf("--id is required")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		data, err := c.doDelete("/v1/admin/apps/" + appRemoveID)
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			AppID          string `json:"app_id"`
			Status         string `json:"status"`
			DeregisteredAt string `json:"deregistered_at"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		printTable(
			[]string{"APP_ID", "STATUS", "DEREGISTERED_AT"},
			[][]string{{resp.AppID, resp.Status, resp.DeregisteredAt}},
		)
		fmt.Fprintln(os.Stderr, "App deregistered. The record is retained; credentials are revoked.")
		return nil
	},
}

func init() {
	appRegisterCmd.Flags().StringVar(&appRegisterName, "name", "", "app name (required)")
	appRegisterCmd.Flags().StringVar(&appRegisterScopes, "scopes", "", "comma-separated scope ceiling (required)")
	appRegisterCmd.Flags().IntVar(&appRegisterTokenTTL, "token-ttl", 0, "app JWT TTL in seconds (default: global AA_APP_TOKEN_TTL)")

	appUpdateCmd.Flags().StringVar(&appUpdateID, "id", "", "app ID to update (required)")
	appUpdateCmd.Flags().StringVar(&appUpdateScopes, "scopes", "", "comma-separated new scope ceiling")
	appUpdateCmd.Flags().IntVar(&appUpdateTokenTTL, "token-ttl", 0, "new app JWT TTL in seconds")

	appRemoveCmd.Flags().StringVar(&appRemoveID, "id", "", "app ID to deregister (required)")

	appCmd.AddCommand(appRegisterCmd, appListCmd, appGetCmd, appUpdateCmd, appRemoveCmd)
	rootCmd.AddCommand(appCmd)
}
