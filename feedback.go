package main

import (
	"encoding/json"
	"fmt"
	"io"
)

const feedbackMediaType = "application/vnd.odyssey.feedback+json"

// resolveExercise finds the exercise to target for feedback.
// If explicitID > 0, fetches that exercise directly.
// Otherwise follows: active_exercise → latest_verified_exercise → error.
func resolveExercise(client *Client, explicitID int) (*ServerResponse, error) {
	if explicitID > 0 {
		path := fmt.Sprintf("/exercise/%d/", explicitID)
		data, err := client.Get(path, exerciseMediaType)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", path, err)
		}
		ex, err := ParseResponse(data)
		if err != nil {
			return nil, fmt.Errorf("parse exercise: %w", err)
		}
		return ex, nil
	}

	data, err := client.Get("/api/", rootMediaType)
	if err != nil {
		return nil, fmt.Errorf("GET /api/: %w", err)
	}
	root, err := ParseResponse(data)
	if err != nil {
		return nil, fmt.Errorf("parse root: %w", err)
	}

	for _, linkName := range []string{"active_exercise", "latest_verified_exercise"} {
		link, ok := root.Links[linkName]
		if !ok {
			continue
		}
		exData, err := client.Get(link.Href, exerciseMediaType)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", link.Href, err)
		}
		ex, err := ParseResponse(exData)
		if err != nil {
			return nil, fmt.Errorf("parse exercise: %w", err)
		}
		return ex, nil
	}

	return nil, fmt.Errorf("no exercise found — run `ody start` first, or pass --exercise <id>")
}

// parseFeedbackArgs extracts the message and optional --exercise flag.
func parseFeedbackArgs(args []string) (message string, exerciseID int) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--exercise" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &exerciseID)
			i++
			continue
		}
		if message == "" {
			message = args[i]
		}
	}
	return
}

// runFeedback is the core implementation of ody feedback.
func runFeedback(client *Client, message string, explicitID int, w io.Writer) error {
	ex, err := resolveExercise(client, explicitID)
	if err != nil {
		return err
	}

	feedbackLink, ok := ex.Links["feedback"]
	if !ok {
		id := exerciseID(ex)
		feedbackLink = Link{Href: fmt.Sprintf("/exercise/%d/feedback/", id)}
	}

	payload, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return fmt.Errorf("marshal feedback: %w", err)
	}

	_, err = client.Post(feedbackLink.Href, feedbackMediaType, payload)
	if err != nil {
		return fmt.Errorf("POST feedback: %w", err)
	}

	fmt.Fprintf(w, "feedback recorded for exercise #%d\n", exerciseID(ex))
	return nil
}
