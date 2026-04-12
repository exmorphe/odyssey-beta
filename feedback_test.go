package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFeedbackWithActiveExercise(t *testing.T) {
	var gotBody map[string]any
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/" && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "root",
				"_links": map[string]any{
					"active_exercise": map[string]any{"href": "/exercise/42/", "method": "GET"},
				},
			})
		case r.URL.Path == "/exercise/42/" && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "exercise", "id": float64(42),
				"_links": map[string]any{
					"self":     map[string]any{"href": "/exercise/42/"},
					"feedback": map[string]any{"href": "/exercise/42/feedback/", "method": "POST"},
				},
			})
		case r.URL.Path == "/exercise/42/feedback/" && r.Method == "POST":
			gotPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "exercise_id": 42, "text": gotBody["text"]})
		}
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var buf strings.Builder

	err := runFeedback(client, "great exercise", 0, &buf)
	if err != nil {
		t.Fatalf("runFeedback: %v", err)
	}
	if gotPath != "/exercise/42/feedback/" {
		t.Errorf("POST path = %q", gotPath)
	}
	if gotBody["text"] != "great exercise" {
		t.Errorf("body text = %v", gotBody["text"])
	}
	if !strings.Contains(buf.String(), "feedback recorded for exercise #42") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestFeedbackFallsBackToLatestVerified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/" && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "root",
				"_links": map[string]any{
					"latest_verified_exercise": map[string]any{"href": "/exercise/99/", "method": "GET"},
				},
			})
		case r.URL.Path == "/exercise/99/" && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "exercise", "id": float64(99),
				"_links": map[string]any{
					"self":     map[string]any{"href": "/exercise/99/"},
					"feedback": map[string]any{"href": "/exercise/99/feedback/", "method": "POST"},
				},
			})
		case r.URL.Path == "/exercise/99/feedback/" && r.Method == "POST":
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "exercise_id": 99, "text": "ok"})
		}
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var buf strings.Builder

	err := runFeedback(client, "ok", 0, &buf)
	if err != nil {
		t.Fatalf("runFeedback: %v", err)
	}
	if !strings.Contains(buf.String(), "exercise #99") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestFeedbackNoExerciseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"_type": "root", "_links": map[string]any{}})
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var buf strings.Builder

	err := runFeedback(client, "hello", 0, &buf)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no exercise found") {
		t.Errorf("error = %v", err)
	}
}

func TestFeedbackExplicitExerciseOverride(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/exercise/77/" && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "exercise", "id": float64(77),
				"_links": map[string]any{
					"self":     map[string]any{"href": "/exercise/77/"},
					"feedback": map[string]any{"href": "/exercise/77/feedback/", "method": "POST"},
				},
			})
		case r.URL.Path == "/exercise/77/feedback/" && r.Method == "POST":
			gotPath = r.URL.Path
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "exercise_id": 77, "text": "override"})
		}
	}))
	defer srv.Close()

	cfg := Config{Server: srv.URL, AccessToken: "test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var buf strings.Builder

	err := runFeedback(client, "override", 77, &buf)
	if err != nil {
		t.Fatalf("runFeedback: %v", err)
	}
	if gotPath != "/exercise/77/feedback/" {
		t.Errorf("POST path = %q", gotPath)
	}
	if !strings.Contains(buf.String(), "exercise #77") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestParseFeedbackArgs(t *testing.T) {
	msg, id := parseFeedbackArgs([]string{"hello world"})
	if msg != "hello world" || id != 0 {
		t.Errorf("basic: msg=%q id=%d", msg, id)
	}

	msg, id = parseFeedbackArgs([]string{"--exercise", "42", "my feedback"})
	if msg != "my feedback" || id != 42 {
		t.Errorf("with flag: msg=%q id=%d", msg, id)
	}

	msg, id = parseFeedbackArgs([]string{"my feedback", "--exercise", "42"})
	if msg != "my feedback" || id != 42 {
		t.Errorf("flag after msg: msg=%q id=%d", msg, id)
	}

	msg, id = parseFeedbackArgs([]string{"this", "is", "great"})
	if msg != "this is great" || id != 0 {
		t.Errorf("unquoted words: msg=%q id=%d", msg, id)
	}

	msg, id = parseFeedbackArgs([]string{"this", "is", "--exercise", "7", "great"})
	if msg != "this is great" || id != 7 {
		t.Errorf("words with flag: msg=%q id=%d", msg, id)
	}
}
