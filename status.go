package main

import (
	"fmt"
	"io"
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

	fmt.Fprintf(w, "Exercise #%d\n", id)
	fmt.Fprintf(w, "  Status:  %s\n", status)
	fmt.Fprintf(w, "  Created: %s\n", createdAt)
	return nil
}
