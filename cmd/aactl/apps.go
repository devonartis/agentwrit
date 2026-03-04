// Package main — aactl app subcommands for managing registered apps.
//
// Commands:
//
//	aactl app register --name NAME --scopes SCOPE_CSV
//	aactl app list [--json]
//	aactl app get APP_ID
//	aactl app update --id APP_ID --scopes SCOPE_CSV
//	aactl app remove --id APP_ID
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// appCmd is the parent command grouping all app-related subcommands
// under "aactl app".
var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage registered apps",
}

// appRegisterName and appRegisterScopes hold flag values for "aactl app register".
var (
	appRegisterName   string
	appRegisterScopes string
)

// appRegisterCmd implements "aactl app register", creating a new app registration
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
			},
		)
		fmt.Fprintln(os.Stderr, "\nWARNING: Save the client_secret — it cannot be retrieved again.")
		return nil
	},
}

// appListCmd implements "aactl app list", printing all registered apps.
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
				a.CreatedAt,
			}
		}
		printTable([]string{"NAME", "APP_ID", "CLIENT_ID", "STATUS", "SCOPES", "CREATED"}, rows)
		fmt.Fprintf(os.Stderr, "Total: %d\n", resp.Total)
		return nil
	},
}

// appGetCmd implements "aactl app get APP_ID", printing full details for one app.
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
				{"CREATED", resp.CreatedAt},
				{"UPDATED", resp.UpdatedAt},
			},
		)
		return nil
	},
}

// appUpdateID and appUpdateScopes hold flag values for "aactl app update".
var (
	appUpdateID     string
	appUpdateScopes string
)

// appUpdateCmd implements "aactl app update --id APP_ID --scopes SCOPE_CSV",
// replacing the scope ceiling for an existing app.
var appUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an app's scope ceiling",
	RunE: func(cmd *cobra.Command, args []string) error {
		if appUpdateID == "" {
			return fmt.Errorf("--id is required")
		}
		if appUpdateScopes == "" {
			return fmt.Errorf("--scopes is required")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		scopes := strings.Split(appUpdateScopes, ",")
		payload := map[string][]string{"scopes": scopes}
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
			UpdatedAt string   `json:"updated_at"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		printTable(
			[]string{"APP_ID", "SCOPES", "UPDATED_AT"},
			[][]string{{resp.AppID, strings.Join(resp.Scopes, ", "), resp.UpdatedAt}},
		)
		return nil
	},
}

// appRemoveID holds the --id flag value for "aactl app remove".
var appRemoveID string

// appRemoveCmd implements "aactl app remove --id APP_ID", deregistering an app
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

	appUpdateCmd.Flags().StringVar(&appUpdateID, "id", "", "app ID to update (required)")
	appUpdateCmd.Flags().StringVar(&appUpdateScopes, "scopes", "", "comma-separated new scope ceiling (required)")

	appRemoveCmd.Flags().StringVar(&appRemoveID, "id", "", "app ID to deregister (required)")

	appCmd.AddCommand(appRegisterCmd, appListCmd, appGetCmd, appUpdateCmd, appRemoveCmd)
	rootCmd.AddCommand(appCmd)
}
