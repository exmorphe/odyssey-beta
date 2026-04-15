package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeExerciseServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "root",
				"_links": map[string]any{
					"active_exercise": map[string]any{
						"href":   "/exercise/42/",
						"method": "GET",
						"title":  "Active exercise",
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
					{"op": "wait", "description": "default ServiceAccount",
						"kubectl":         []string{"get", "serviceaccount", "default", "-n", "exercise"},
						"timeout_seconds": 30},
					{"op": "apply", "manifest": "deployment.yaml", "content": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\n  namespace: exercise\n"},
					{"op": "kubectl", "args": []string{"label", "pod", "-n", "exercise", "-l", "app=myapp", "env=test"}},
				},
				"_links": map[string]any{
					"self": map[string]any{"href": "/exercise/42/", "method": "GET"},
				},
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
}

func TestStartHappyPath(t *testing.T) {
	srv := fakeExerciseServer(t)
	defer srv.Close()
	mock := &MockRunner{
		OutputResults: map[string]string{
			"get namespaces -o jsonpath={.items[*].metadata.name}": "default exercise kube-system kube-public kube-node-lease local-path-storage",
			"get serviceaccount default -n exercise":               `{"metadata":{"name":"default"}}`,
		},
	}
	kind := &MockKindManager{Exists: true}
	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runStart(client, mock, kind, &output)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Verify namespace cleanup: should delete "exercise"
	foundDelete := false
	for _, call := range mock.RunCalls {
		if len(call.Args) >= 3 && call.Args[0] == "delete" && call.Args[1] == "namespace" && call.Args[2] == "exercise" {
			foundDelete = true
		}
	}
	if !foundDelete {
		t.Error("expected namespace 'exercise' to be deleted")
	}

	// Verify apply calls
	applyCount := 0
	for _, call := range mock.RunCalls {
		if len(call.Args) >= 1 && call.Args[0] == "apply" {
			applyCount++
		}
	}
	if applyCount != 2 {
		t.Errorf("apply calls = %d, want 2", applyCount)
	}

	// Verify kubectl args call
	foundLabel := false
	for _, call := range mock.RunCalls {
		if len(call.Args) >= 1 && call.Args[0] == "label" {
			foundLabel = true
		}
	}
	if !foundLabel {
		t.Error("expected kubectl label call")
	}

	out := output.String()
	if !strings.Contains(out, "Exercise #42 applied") {
		t.Errorf("output = %q", out)
	}
	if strings.Contains(out, "steps") {
		t.Errorf("should not show step count: %q", out)
	}
	if !strings.Contains(out, "Namespaces:") {
		t.Errorf("missing namespaces: %q", out)
	}
	if !strings.Contains(out, "Resources:") {
		t.Errorf("missing resources: %q", out)
	}
	if !strings.Contains(out, "ody verify") {
		t.Errorf("missing verify hint: %q", out)
	}
}

func TestStartNoActiveExercise(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"_type": "root", "_links": map[string]any{},
		})
	}))
	defer srv.Close()
	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runStart(client, &MockRunner{}, &MockKindManager{Exists: true}, &output)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !strings.Contains(output.String(), "No active exercise") {
		t.Errorf("output = %q", output.String())
	}
}

func TestStartCreatesKindCluster(t *testing.T) {
	srv := fakeExerciseServer(t)
	defer srv.Close()
	mock := &MockRunner{
		OutputResults: map[string]string{
			"get namespaces -o jsonpath={.items[*].metadata.name}": "default kube-system kube-public kube-node-lease local-path-storage",
			"get serviceaccount default -n exercise":               `{"metadata":{"name":"default"}}`,
		},
	}
	kind := &MockKindManager{Exists: false}
	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runStart(client, mock, kind, &output)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !kind.CreateCalled {
		t.Error("expected kind create cluster to be called")
	}
	if !strings.Contains(output.String(), "Creating kind cluster") {
		t.Errorf("output = %q", output.String())
	}
}

func TestStartHandlesWaitOp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "root",
				"_links": map[string]any{
					"active_exercise": map[string]any{"href": "/exercise/42/", "method": "GET"},
				},
			})
		case "/exercise/42/":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "exercise", "id": 42, "status": "active", "created_at": "2026-04-09T14:00:00Z",
				"steps": []map[string]any{
					{"op": "apply", "manifest": "namespace.yaml", "content": "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: exercise\n"},
					{"op": "wait", "description": "default ServiceAccount",
						"kubectl":         []string{"get", "serviceaccount", "default", "-n", "exercise"},
						"timeout_seconds": 30},
					{"op": "apply", "manifest": "deployment.yaml", "content": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\n  namespace: exercise\n"},
				},
				"_links": map[string]any{"self": map[string]any{"href": "/exercise/42/", "method": "GET"}},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	mock := &MockRunner{
		OutputResults: map[string]string{
			"get namespaces -o jsonpath={.items[*].metadata.name}": "default kube-system kube-public kube-node-lease local-path-storage",
			"get serviceaccount default -n exercise":               `{"metadata":{"name":"default"}}`,
		},
	}
	kind := &MockKindManager{Exists: true}
	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runStart(client, mock, kind, &output)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Verify the wait op triggered an Output call for the SA
	foundSAGet := false
	for _, call := range mock.OutputCalls {
		if len(call) >= 3 && call[0] == "get" && call[1] == "serviceaccount" {
			foundSAGet = true
		}
	}
	if !foundSAGet {
		t.Error("expected kubectl get serviceaccount call from wait op")
	}
}
