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

var ceilingCmd = &cobra.Command{
	Use:   "ceiling",
	Short: "Manage sidecar scope ceilings",
}

var ceilingGetCmd = &cobra.Command{
	Use:   "get [sidecar-id]",
	Short: "Get scope ceiling for a sidecar",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		data, err := c.doGet("/v1/admin/sidecars/" + args[0] + "/ceiling")
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			SidecarID    string   `json:"sidecar_id"`
			ScopeCeiling []string `json:"scope_ceiling"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		printTable(
			[]string{"SIDECAR ID", "SCOPE CEILING"},
			[][]string{{resp.SidecarID, strings.Join(resp.ScopeCeiling, ", ")}},
		)
		return nil
	},
}

var ceilingSetScopes string

var ceilingSetCmd = &cobra.Command{
	Use:   "set [sidecar-id]",
	Short: "Update scope ceiling for a sidecar",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if ceilingSetScopes == "" {
			return fmt.Errorf("--scopes is required")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		scopes := strings.Split(ceilingSetScopes, ",")
		payload := map[string][]string{"scope_ceiling": scopes}
		data, err := c.doPut("/v1/admin/sidecars/"+args[0]+"/ceiling", payload)
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			OldCeiling   []string `json:"old_ceiling"`
			NewCeiling   []string `json:"new_ceiling"`
			Narrowed     bool     `json:"narrowed"`
			Revoked      bool     `json:"revoked"`
			RevokedCount int      `json:"revoked_count"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		printTable(
			[]string{"OLD CEILING", "NEW CEILING", "NARROWED", "REVOKED", "REVOKED COUNT"},
			[][]string{{
				strings.Join(resp.OldCeiling, ","),
				strings.Join(resp.NewCeiling, ","),
				fmt.Sprintf("%v", resp.Narrowed),
				fmt.Sprintf("%v", resp.Revoked),
				fmt.Sprintf("%d", resp.RevokedCount),
			}},
		)
		return nil
	},
}

func init() {
	sidecarsCmd.AddCommand(sidecarsListCmd)
	rootCmd.AddCommand(sidecarsCmd)
}

func init() {
	ceilingSetCmd.Flags().StringVar(&ceilingSetScopes, "scopes", "", "comma-separated scope ceiling (required)")
	ceilingCmd.AddCommand(ceilingGetCmd, ceilingSetCmd)
	sidecarsCmd.AddCommand(ceilingCmd)
}
