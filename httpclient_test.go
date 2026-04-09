package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientGetSendsBearerAndAccept(t *testing.T) {
	var gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.Write([]byte(`{"_type":"root","_links":{}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := Config{
		Server:       srv.URL,
		AccessToken:  "at-test",
		RefreshToken: "rt-test",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	SaveConfig(dir, cfg)
	c := NewClient(cfg, dir)

	_, err := c.Get("/api/", "application/vnd.odyssey.root+json")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if gotAuth != "Bearer at-test" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotAccept != "application/vnd.odyssey.root+json" {
		t.Errorf("accept = %q", gotAccept)
	}
}

func TestClientPostSendsContentType(t *testing.T) {
	var gotCT string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"_type":"verification","status":"failing"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := Config{
		Server:      srv.URL,
		AccessToken: "at-test",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	c := NewClient(cfg, dir)

	_, err := c.Post("/exercise/42/", "application/vnd.odyssey.cluster-snapshot+json", []byte(`{"items":[]}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if gotCT != "application/vnd.odyssey.cluster-snapshot+json" {
		t.Errorf("content-type = %q", gotCT)
	}
	var body map[string]any
	json.Unmarshal(gotBody, &body)
	if _, ok := body["items"]; !ok {
		t.Errorf("body missing items key")
	}
}

func TestClientRefreshesExpiredToken(t *testing.T) {
	refreshCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token/" {
			refreshCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "at-refreshed",
				"refresh_token": "rt-refreshed",
				"expires_in":    3600,
			})
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer at-refreshed" {
			t.Errorf("expected refreshed token, got %q", auth)
		}
		w.Write([]byte(`{"_type":"root","_links":{}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := Config{
		Server:       srv.URL,
		AccessToken:  "at-expired",
		RefreshToken: "rt-old",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}
	SaveConfig(dir, cfg)
	c := NewClient(cfg, dir)

	_, err := c.Get("/api/", "")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !refreshCalled {
		t.Error("refresh was not called")
	}

	loaded, _ := LoadConfig(dir)
	if loaded.AccessToken != "at-refreshed" {
		t.Errorf("persisted token = %q", loaded.AccessToken)
	}
}

func TestClientRefreshFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token/" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := Config{
		Server:       srv.URL,
		AccessToken:  "at-expired",
		RefreshToken: "rt-bad",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}
	c := NewClient(cfg, dir)

	_, err := c.Get("/api/", "")
	if err == nil {
		t.Fatal("expected error on refresh failure")
	}
}

func TestClientHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	cfg := Config{
		Server:      srv.URL,
		AccessToken: "at-test",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	c := NewClient(cfg, t.TempDir())

	_, err := c.Get("/api/", "")
	if err == nil {
		t.Fatal("expected error on 500")
	}
}
