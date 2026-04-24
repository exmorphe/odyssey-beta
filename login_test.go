package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginHappyPath(t *testing.T) {
	oldShould := shouldAutoOpen
	shouldAutoOpen = func() bool { return false }
	t.Cleanup(func() { shouldAutoOpen = oldShould })

	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device-authorization/":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "dev-abc",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://example.com/activate",
				"interval":         1,
			})
		case "/oauth/token/":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "device_code=dev-abc") {
				t.Errorf("missing device_code in token request")
			}
			pollCount++
			if pollCount < 2 {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "authorization_pending",
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "at-new",
				"refresh_token": "rt-new",
				"expires_in":    3600,
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	var output strings.Builder

	err := runLogin(srv.URL, dir, &output)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "ABCD-1234") {
		t.Errorf("output missing user code: %s", out)
	}

	cfg, _ := LoadConfig(dir)
	if cfg.Server != srv.URL {
		t.Errorf("server = %q", cfg.Server)
	}
	if cfg.AccessToken != "at-new" {
		t.Errorf("access_token = %q", cfg.AccessToken)
	}
	if cfg.RefreshToken != "rt-new" {
		t.Errorf("refresh_token = %q", cfg.RefreshToken)
	}
}

func TestLoginDeviceEndpointError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	var output strings.Builder
	err := runLogin(srv.URL, t.TempDir(), &output)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoginTokenDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device-authorization/":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "dev-abc",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://example.com/activate",
				"interval":         1,
			})
		case "/oauth/token/":
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "access_denied",
			})
		}
	}))
	defer srv.Close()

	var output strings.Builder
	err := runLogin(srv.URL, t.TempDir(), &output)
	if err == nil {
		t.Fatal("expected error on access_denied")
	}
}
