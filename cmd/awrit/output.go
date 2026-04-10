package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// printJSON pretty-prints raw JSON bytes to stdout with 2-space indentation.
// Falls back to printing raw bytes if indentation fails.
func printJSON(data []byte) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		fmt.Fprintln(os.Stdout, string(data))
		return
	}
	fmt.Fprintln(os.Stdout, pretty.String())
}

// printTable writes a tabwriter-formatted table to stdout with the given
// column headers and row data.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}
