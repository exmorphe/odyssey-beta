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

func TestParseNamespacesFromSteps(t *testing.T) {
	steps := []Step{
		{Op: "apply", Manifest: "namespace.yaml", Content: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: exercise\n"},
		{Op: "apply", Manifest: "ns2.yaml", Content: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: other\n"},
		{Op: "apply", Manifest: "deploy.yaml", Content: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\n  namespace: exercise\n"},
		{Op: "kubectl", Args: []string{"label", "pod"}},
	}
	ns := parseNamespaces(steps)
	if len(ns) != 2 {
		t.Fatalf("namespaces = %v, want 2 items", ns)
	}
	expected := map[string]bool{"exercise": true, "other": true}
	for _, n := range ns {
		if !expected[n] {
			t.Errorf("unexpected namespace: %s", n)
		}
	}
}

func TestParseKindsFromSteps(t *testing.T) {
	steps := []Step{
		{Op: "apply", Manifest: "namespace.yaml", Content: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: exercise\n"},
		{Op: "apply", Manifest: "deploy.yaml", Content: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\n"},
		{Op: "apply", Manifest: "svc.yaml", Content: "apiVersion: v1\nkind: Service\nmetadata:\n  name: mysvc\n"},
		{Op: "kubectl", Args: []string{"label", "pod"}},
	}
	kinds := parseKinds(steps)
	expected := map[string]bool{"Deployment": true, "Service": true}
	for _, k := range kinds {
		if !expected[k] {
			t.Errorf("unexpected kind: %s", k)
		}
	}
	if len(kinds) != 2 {
		t.Errorf("kinds = %v", kinds)
	}
}

func TestVerifyHappyPath(t *testing.T) {
	var gotSnapshot map[string]any
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
			if r.Method == "POST" {
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &gotSnapshot)
				json.NewEncoder(w).Encode(map[string]any{
					"_type":  "verification",
					"status": "failing",
					"checks": []map[string]any{
						{"check": "image_check", "target": "deploy/exercise/myapp", "result": "FAIL"},
					},
					"faults": []map[string]any{
						{"fault_key": "wrong_image/tag_mismatch", "fault_ids": []string{"fault-07"}, "result": "FAIL", "masking": "visible", "masked_by": nil, "symptom": "image pull error"},
						{"fault_key": "missing_labels/no_selector_match", "fault_ids": []string{"fault-12"}, "result": "FAIL", "masking": "masked", "masked_by": "wrong_image/tag_mismatch", "symptom": "pods not ready"},
					},
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "exercise", "id": 42, "status": "active", "created_at": "2026-04-09T14:00:00Z",
				"steps": []map[string]any{
					{"op": "apply", "manifest": "namespace.yaml", "content": "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: exercise\n"},
					{"op": "apply", "manifest": "deployment.yaml", "content": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\n  namespace: exercise\n"},
				},
				"_links": map[string]any{"self": map[string]any{"href": "/exercise/42/", "method": "GET"}},
			})
		}
	}))
	defer srv.Close()

	mock := &MockRunner{
		OutputResults: map[string]string{
			"get Deployment -n exercise -o json": `{"apiVersion":"v1","items":[{"kind":"Deployment","metadata":{"name":"myapp"}}]}`,
		},
	}
	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runVerify(client, mock, &output)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "wrong_image/tag_mismatch") {
		t.Errorf("missing fault key: %s", out)
	}
	if !strings.Contains(out, "FAIL") {
		t.Errorf("missing FAIL: %s", out)
	}
	if !strings.Contains(out, "masked by wrong_image") {
		t.Errorf("missing masking: %s", out)
	}
	if !strings.Contains(out, "0/2 faults resolved") {
		t.Errorf("missing summary: %s", out)
	}

	// Verify snapshot payload was sent with correct structure
	if gotSnapshot == nil {
		t.Fatal("snapshot was not captured")
	}
	if _, ok := gotSnapshot["Deployment/exercise"]; !ok {
		t.Errorf("snapshot missing Deployment/exercise key, got keys: %v", gotSnapshot)
	}
}

func TestVerifySolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "root", "_links": map[string]any{
					"active_exercise": map[string]any{"href": "/exercise/42/", "method": "GET"},
				},
			})
		case "/exercise/42/":
			if r.Method == "POST" {
				json.NewEncoder(w).Encode(map[string]any{
					"_type": "verification", "status": "solved", "checks": []map[string]any{},
					"faults": []map[string]any{
						{"fault_key": "wrong_image/tag_mismatch", "fault_ids": []string{"fault-07"}, "result": "PASS", "masking": "visible", "masked_by": nil},
					},
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"_type": "exercise", "id": 42, "status": "active", "created_at": "2026-04-09T14:00:00Z",
				"steps": []map[string]any{
					{"op": "apply", "manifest": "ns.yaml", "content": "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: exercise\n"},
					{"op": "apply", "manifest": "deploy.yaml", "content": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: myapp\n"},
				},
				"_links": map[string]any{"self": map[string]any{"href": "/exercise/42/", "method": "GET"}},
			})
		}
	}))
	defer srv.Close()

	mock := &MockRunner{
		OutputResults: map[string]string{
			"get Deployment -n exercise -o json": `{"items":[]}`,
		},
	}
	cfg := Config{Server: srv.URL, AccessToken: "at-test", ExpiresAt: time.Now().Add(time.Hour)}
	client := NewClient(cfg, t.TempDir())
	var output strings.Builder

	err := runVerify(client, mock, &output)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(output.String(), "Solved!") {
		t.Errorf("output = %q", output.String())
	}
}
