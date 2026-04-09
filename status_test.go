package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStatusHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "root",
				"_links": map[string]any{
					"active_exercise": map[string]any{
						"href": "/exercise/42/", "method": "GET",
					},
				},
			})
		case "/exercise/42/":
			json.NewEncoder(w).Encode(map[string]any{
				"_type":      "exercise",
				"id":         42,
				"status":     "active",
				"created_at": "2026-04-09T14:00:00Z",
				"steps":      []any{},
				"_links":     map[string]any{},
			})
		}
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runStatus(client, &output)
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "42") {
		t.Errorf("missing exercise ID: %s", out)
	}
	if !strings.Contains(out, "active") {
		t.Errorf("missing status: %s", out)
	}
	if !strings.Contains(out, "2026-04-09") {
		t.Errorf("missing created_at: %s", out)
	}
}

func TestStatusNoActiveExercise(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"_type": "root", "_links": map[string]any{},
		})
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runStatus(client, &output)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(output.String(), "No active exercise") {
		t.Errorf("output = %q", output.String())
	}
}

func TestStatusSolvedExercise(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "root",
				"_links": map[string]any{
					"active_exercise": map[string]any{
						"href": "/exercise/42/", "method": "GET",
					},
				},
			})
		case "/exercise/42/":
			json.NewEncoder(w).Encode(map[string]any{
				"_type":      "exercise",
				"id":         42,
				"status":     "solved",
				"created_at": "2026-04-09T14:00:00Z",
				"steps":      []any{},
				"_links":     map[string]any{},
			})
		}
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runStatus(client, &output)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(output.String(), "solved") {
		t.Errorf("output = %q", output.String())
	}
}
