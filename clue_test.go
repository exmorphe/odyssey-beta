package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClueHappyPath(t *testing.T) {
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
				"_type":  "exercise",
				"id":     42,
				"status": "active",
				"steps":  []any{},
				"probes": []map[string]any{
					{"pod": "traffic-probe", "namespace": "exercise"},
				},
				"_links": map[string]any{},
			})
		}
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "tok", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())

	mock := &MockRunner{
		OutputResults: map[string]string{
			"logs traffic-probe -n exercise --tail=10": "2026-04-11T10:00:00+00:00 GET http://web-api:80/\nHTTP 000 (3.001s)\nCONNECT FAILED\n",
		},
	}

	var output strings.Builder
	err := runClue(client, mock, &output)
	if err != nil {
		t.Fatalf("clue: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "kubectl logs traffic-probe -n exercise --tail=10") {
		t.Errorf("should show kubectl command: %s", out)
	}
	if !strings.Contains(out, "CONNECT FAILED") {
		t.Errorf("should show probe output: %s", out)
	}
	if len(mock.OutputCalls) != 1 {
		t.Fatalf("expected 1 kubectl call, got %d", len(mock.OutputCalls))
	}
	expected := []string{"logs", "traffic-probe", "-n", "exercise", "--tail=10"}
	for i, arg := range expected {
		if mock.OutputCalls[0][i] != arg {
			t.Errorf("kubectl arg %d: got %q, want %q", i, mock.OutputCalls[0][i], arg)
		}
	}
}

func TestClueNoProbes(t *testing.T) {
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
				"_type":  "exercise",
				"id":     42,
				"status": "active",
				"steps":  []any{},
				"probes": []any{},
				"_links": map[string]any{},
			})
		}
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "tok", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())

	var output strings.Builder
	err := runClue(client, &MockRunner{}, &output)
	if err != nil {
		t.Fatalf("clue: %v", err)
	}
	if !strings.Contains(output.String(), "No clues available") {
		t.Errorf("output = %q", output.String())
	}
}

func TestClueNoActiveExercise(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"_type": "root", "_links": map[string]any{},
		})
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "tok", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())

	var output strings.Builder
	err := runClue(client, &MockRunner{}, &output)
	if err != nil {
		t.Fatalf("clue: %v", err)
	}
	if !strings.Contains(output.String(), "No active exercise") {
		t.Errorf("output = %q", output.String())
	}
}

func TestClueMultipleProbes(t *testing.T) {
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
				"_type":  "exercise",
				"id":     42,
				"status": "active",
				"steps":  []any{},
				"probes": []map[string]any{
					{"pod": "probe-a", "namespace": "ns1"},
					{"pod": "probe-b", "namespace": "ns2"},
				},
				"_links": map[string]any{},
			})
		}
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "tok", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())

	mock := &MockRunner{
		OutputResults: map[string]string{
			"logs probe-a -n ns1 --tail=10": "log line a\n",
			"logs probe-b -n ns2 --tail=10": "log line b\n",
		},
	}

	var output strings.Builder
	err := runClue(client, mock, &output)
	if err != nil {
		t.Fatalf("clue: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "probe-a") {
		t.Errorf("missing probe-a: %s", out)
	}
	if !strings.Contains(out, "probe-b") {
		t.Errorf("missing probe-b: %s", out)
	}
	if len(mock.OutputCalls) != 2 {
		t.Errorf("expected 2 kubectl calls, got %d", len(mock.OutputCalls))
	}
}
