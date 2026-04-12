package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// betaKey is sent as X-Beta-Key on every request to pass the Caddy gate.
// Hardcoded because the odyssey-beta repo is private.
const betaKey = "odyssey-beta-2026"

// Client is an HTTP client that handles OAuth tokens and content negotiation.
type Client struct {
	cfg     Config
	dir     string
	httpCli *http.Client
}

// NewClient creates a new Client with the given config and state directory.
func NewClient(cfg Config, dir string) *Client {
	return &Client{
		cfg:     cfg,
		dir:     dir,
		httpCli: &http.Client{Timeout: 30 * time.Second},
	}
}

// Get performs an authenticated GET request with the given Accept header.
func (c *Client) Get(path, accept string) ([]byte, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", c.cfg.Server+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return c.doRequest(req)
}

// Post performs an authenticated POST request with the given content type and body.
func (c *Client) Post(path, contentType string, body []byte) ([]byte, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.cfg.Server+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	req.Header.Set("Content-Type", contentType)
	return c.doRequest(req)
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("X-Beta-Key", betaKey)

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	const maxResponseBytes = 10 << 20 // 10 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func (c *Client) ensureValidToken() error {
	if !c.cfg.IsExpired() {
		return nil
	}
	return c.refreshToken()
}

func (c *Client) refreshToken() error {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.cfg.RefreshToken},
		"client_id":     {clientID},
	}
	resp, err := betaPostForm(c.httpCli, c.cfg.Server+"/oauth/token/", form)
	if err != nil {
		return fmt.Errorf("token refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expired — run `ody login` (refresh failed: HTTP %d: %s)", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}

	c.cfg.AccessToken = tokenResp.AccessToken
	c.cfg.RefreshToken = tokenResp.RefreshToken
	c.cfg.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	if err := SaveConfig(c.dir, c.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}
