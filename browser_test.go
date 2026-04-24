package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAutoOpenAllowed(t *testing.T) {
	t.Run("NO_BROWSER set disables regardless of tty", func(t *testing.T) {
		t.Setenv("NO_BROWSER", "1")
		if autoOpenAllowed(true) {
			t.Error("NO_BROWSER set + tty should be false")
		}
		if autoOpenAllowed(false) {
			t.Error("NO_BROWSER set + non-tty should be false")
		}
	})
	t.Run("empty NO_BROWSER with tty allows", func(t *testing.T) {
		t.Setenv("NO_BROWSER", "")
		if !autoOpenAllowed(true) {
			t.Error("empty NO_BROWSER + tty should be true")
		}
		if autoOpenAllowed(false) {
			t.Error("empty NO_BROWSER + non-tty should be false")
		}
	})
}

func deviceServerWithComplete(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device-authorization/":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dev-abc",
				"user_code":                 "ABCD-1234",
				"verification_uri":          "/oauth/device/",
				"verification_uri_complete": "/oauth/device/?user_code=ABCD-1234",
				"interval":                  1,
			})
		case "/oauth/token/":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "at", "refresh_token": "rt", "expires_in": 3600,
			})
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestLoginOpensCompleteURL(t *testing.T) {
	var openedURL string
	oldOpen := openBrowser
	openBrowser = func(u string) error { openedURL = u; return nil }
	t.Cleanup(func() { openBrowser = oldOpen })

	oldShould := shouldAutoOpen
	shouldAutoOpen = func() bool { return true }
	t.Cleanup(func() { shouldAutoOpen = oldShould })

	srv := deviceServerWithComplete(t)
	defer srv.Close()

	var out strings.Builder
	if err := runLogin(srv.URL, t.TempDir(), &out); err != nil {
		t.Fatal(err)
	}
	want := srv.URL + "/oauth/device/?user_code=ABCD-1234"
	if openedURL != want {
		t.Errorf("openedURL = %q, want %q", openedURL, want)
	}
	if !strings.Contains(out.String(), "Opening browser") {
		t.Error("expected 'Opening browser' message")
	}
}

func TestLoginSkipsWhenShouldAutoOpenFalse(t *testing.T) {
	called := false
	oldOpen := openBrowser
	openBrowser = func(u string) error { called = true; return nil }
	t.Cleanup(func() { openBrowser = oldOpen })

	oldShould := shouldAutoOpen
	shouldAutoOpen = func() bool { return false }
	t.Cleanup(func() { shouldAutoOpen = oldShould })

	srv := deviceServerWithComplete(t)
	defer srv.Close()

	var out strings.Builder
	if err := runLogin(srv.URL, t.TempDir(), &out); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("openBrowser should not be called")
	}
	if strings.Contains(out.String(), "Opening browser") {
		t.Error("should not announce browser open when skipped")
	}
	if !strings.Contains(out.String(), "ABCD-1234") {
		t.Error("fallback text with user code must still be printed")
	}
}

func TestLoginContinuesWhenOpenerFails(t *testing.T) {
	oldOpen := openBrowser
	openBrowser = func(u string) error { return os.ErrNotExist }
	t.Cleanup(func() { openBrowser = oldOpen })

	oldShould := shouldAutoOpen
	shouldAutoOpen = func() bool { return true }
	t.Cleanup(func() { shouldAutoOpen = oldShould })

	srv := deviceServerWithComplete(t)
	defer srv.Close()

	var out strings.Builder
	if err := runLogin(srv.URL, t.TempDir(), &out); err != nil {
		t.Fatalf("login should succeed even if opener fails: %v", err)
	}
	if strings.Contains(out.String(), "Opening browser") {
		t.Error("should not announce browser open on failure")
	}
	if !strings.Contains(out.String(), "ABCD-1234") {
		t.Error("fallback text with user code must still be printed")
	}
}

func TestLoginFallsBackToPlainVerificationURI(t *testing.T) {
	var openedURL string
	oldOpen := openBrowser
	openBrowser = func(u string) error { openedURL = u; return nil }
	t.Cleanup(func() { openBrowser = oldOpen })

	oldShould := shouldAutoOpen
	shouldAutoOpen = func() bool { return true }
	t.Cleanup(func() { shouldAutoOpen = oldShould })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device-authorization/":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "dev-abc",
				"user_code":        "ABCD-1234",
				"verification_uri": "/oauth/device/",
				"interval":         1,
			})
		case "/oauth/token/":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "at", "refresh_token": "rt", "expires_in": 3600,
			})
		}
	}))
	defer srv.Close()

	var out strings.Builder
	if err := runLogin(srv.URL, t.TempDir(), &out); err != nil {
		t.Fatal(err)
	}
	want := srv.URL + "/oauth/device/"
	if openedURL != want {
		t.Errorf("openedURL = %q, want %q", openedURL, want)
	}
}
