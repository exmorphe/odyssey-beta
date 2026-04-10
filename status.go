package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// runStatus displays the current exercise state.
func runStatus(client *Client, w io.Writer) error {
	ex, err := fetchExercise(client)
	if err != nil {
		return err
	}
	if ex == nil {
		fmt.Fprintln(w, "No active exercise — generate one in the browser.")
		return nil
	}

	id := exerciseID(ex)
	statusRaw, _ := ex.Field("status")
	status, _ := statusRaw.(string)
	createdRaw, _ := ex.Field("created_at")
	createdAt, _ := createdRaw.(string)

	formatted := formatTimestamp(createdAt)

	fmt.Fprintf(w, "Exercise #%d\n", id)
	fmt.Fprintf(w, "  Status:     %s\n", status)
	fmt.Fprintf(w, "  Created:    %s\n", formatted)

	steps, err := exerciseSteps(ex)
	if err != nil {
		return err
	}
	if len(steps) > 0 {
		if ns := parseNamespaces(steps); len(ns) > 0 {
			fmt.Fprintf(w, "  Namespaces: %s\n", strings.Join(ns, ", "))
		}
		if kinds := parseKinds(steps); len(kinds) > 0 {
			fmt.Fprintf(w, "  Resources:  %s\n", strings.Join(kinds, ", "))
		}
	}

	return nil
}

// formatTimestamp parses an RFC3339 timestamp and formats it as "02 Jan 2006 15:04 UTC".
// Falls back to the raw string if parsing fails.
func formatTimestamp(raw string) string {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return raw
	}
	return t.UTC().Format("02 Jan 2006 15:04 UTC")
}
