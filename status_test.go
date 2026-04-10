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
				"steps": []map[string]any{
					{"op": "apply", "manifest": "namespace.yaml", "content": "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: exercise\n"},
					{"op": "apply", "manifest": "deployment.yaml", "content": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\n  namespace: exercise\n"},
					{"op": "apply", "manifest": "service.yaml", "content": "apiVersion: v1\nkind: Service\nmetadata:\n  name: mysvc\n  namespace: exercise\n"},
				},
				"_links": map[string]any{},
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
	if !strings.Contains(out, "09 Apr 2026 14:00 UTC") {
		t.Errorf("missing formatted timestamp: %s", out)
	}
	if !strings.Contains(out, "Namespaces:") {
		t.Errorf("missing Namespaces label: %s", out)
	}
	if !strings.Contains(out, "exercise") {
		t.Errorf("missing namespace 'exercise': %s", out)
	}
	if !strings.Contains(out, "Resources:") {
		t.Errorf("missing Resources label: %s", out)
	}
	if !strings.Contains(out, "Deployment") {
		t.Errorf("missing resource kind 'Deployment': %s", out)
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
