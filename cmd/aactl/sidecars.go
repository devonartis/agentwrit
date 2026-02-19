package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var sidecarsCmd = &cobra.Command{
	Use:   "sidecars",
	Short: "Manage sidecars",
}

var sidecarsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered sidecars",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		data, err := c.doGet("/v1/admin/sidecars")
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			Sidecars []struct {
				SidecarID    string   `json:"sidecar_id"`
				ScopeCeiling []string `json:"scope_ceiling"`
				Status       string   `json:"status"`
				CreatedAt    string   `json:"created_at"`
				UpdatedAt    string   `json:"updated_at"`
			} `json:"sidecars"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		rows := make([][]string, len(resp.Sidecars))
		for i, s := range resp.Sidecars {
			rows[i] = []string{
				s.SidecarID,
				strings.Join(s.ScopeCeiling, ","),
				s.Status,
				s.CreatedAt,
			}
		}
		printTable([]string{"ID", "SCOPES", "STATUS", "CREATED"}, rows)
		fmt.Fprintf(os.Stderr, "Total: %d\n", resp.Total)
		return nil
	},
}

func init() {
	sidecarsCmd.AddCommand(sidecarsListCmd)
	rootCmd.AddCommand(sidecarsCmd)
}
