// audit.go implements the aactl audit command group.
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// auditCmd is the parent command for all audit subcommands.
var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Query audit trail",
}

// auditAgentID filters events by agent SPIFFE ID.
var auditAgentID string

// auditTaskID filters events by task ID.
var auditTaskID string

// auditEventType filters events by event type (e.g. token_revoked).
var auditEventType string

// auditSince filters events to those after this RFC3339 timestamp.
var auditSince string

// auditUntil filters events to those before this RFC3339 timestamp.
var auditUntil string

// auditLimit sets the maximum number of events to return.
var auditLimit int

// auditOffset sets the pagination offset.
var auditOffset int

// auditEventsCmd lists audit events with optional filters.
// Output is a table (default) or raw JSON (--json flag).
// Detail column is truncated to 60 characters for readability.
var auditEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "List audit events with optional filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}

		params := url.Values{}
		if auditAgentID != "" {
			params.Set("agent_id", auditAgentID)
		}
		if auditTaskID != "" {
			params.Set("task_id", auditTaskID)
		}
		if auditEventType != "" {
			params.Set("event_type", auditEventType)
		}
		if auditSince != "" {
			params.Set("since", auditSince)
		}
		if auditUntil != "" {
			params.Set("until", auditUntil)
		}
		if auditLimit > 0 {
			params.Set("limit", strconv.Itoa(auditLimit))
		}
		if auditOffset > 0 {
			params.Set("offset", strconv.Itoa(auditOffset))
		}

		path := "/v1/audit/events"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}

		data, err := c.doGet(path)
		if err != nil {
			return err
		}
		if jsonOutput {
			printJSON(data)
			return nil
		}

		var resp struct {
			Events []struct {
				ID        string `json:"id"`
				Timestamp string `json:"timestamp"`
				EventType string `json:"event_type"`
				AgentID   string `json:"agent_id"`
				Detail    string `json:"detail"`
			} `json:"events"`
			Total  int `json:"total"`
			Offset int `json:"offset"`
			Limit  int `json:"limit"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		rows := make([][]string, len(resp.Events))
		for i, e := range resp.Events {
			detail := e.Detail
			if len(detail) > 60 {
				detail = detail[:57] + "..."
			}
			rows[i] = []string{e.ID, e.Timestamp, e.EventType, e.AgentID, detail}
		}
		printTable([]string{"ID", "TIMESTAMP", "EVENT TYPE", "AGENT ID", "DETAIL"}, rows)
		fmt.Fprintf(os.Stderr, "Showing %d of %d events (offset=%d, limit=%d)\n",
			len(resp.Events), resp.Total, resp.Offset, resp.Limit)
		return nil
	},
}

func init() {
	auditEventsCmd.Flags().StringVar(&auditAgentID, "agent-id", "", "filter by agent SPIFFE ID")
	auditEventsCmd.Flags().StringVar(&auditTaskID, "task-id", "", "filter by task ID")
	auditEventsCmd.Flags().StringVar(&auditEventType, "event-type", "", "filter by event type")
	auditEventsCmd.Flags().StringVar(&auditSince, "since", "", "events after this time (RFC3339)")
	auditEventsCmd.Flags().StringVar(&auditUntil, "until", "", "events before this time (RFC3339)")
	auditEventsCmd.Flags().IntVar(&auditLimit, "limit", 0, "max results (default 100)")
	auditEventsCmd.Flags().IntVar(&auditOffset, "offset", 0, "pagination offset")
	auditCmd.AddCommand(auditEventsCmd)
	rootCmd.AddCommand(auditCmd)
}
