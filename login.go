package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var errSlowDown = errors.New("slow_down")

const clientID = "ody-cli"

func runLogin(serverURL string, configDir string, w io.Writer) error {
	httpCli := &http.Client{Timeout: 30 * time.Second}

	resp, err := httpCli.PostForm(serverURL+"/oauth/device/", url.Values{
		"client_id": {clientID},
	})
	if err != nil {
		return fmt.Errorf("device authorization request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device authorization failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var deviceResp struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return fmt.Errorf("decode device response: %w", err)
	}

	fmt.Fprintf(w, "Visit %s and enter code: %s\n", deviceResp.VerificationURI, deviceResp.UserCode)

	interval := time.Duration(deviceResp.Interval) * time.Second
	if interval < time.Second {
		interval = time.Second
	}

	for {
		time.Sleep(interval)

		tokenResp, done, err := pollToken(httpCli, serverURL, deviceResp.DeviceCode)
		if errors.Is(err, errSlowDown) {
			interval += 5 * time.Second
			continue
		}
		if err != nil {
			return err
		}
		if !done {
			continue
		}

		cfg := Config{
			Server:       serverURL,
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		}
		if err := SaveConfig(configDir, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Fprintf(w, "Logged in to %s\n", serverURL)
		return nil
	}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func pollToken(httpCli *http.Client, serverURL, deviceCode string) (*tokenResponse, bool, error) {
	resp, err := httpCli.PostForm(serverURL+"/oauth/token/", url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
		"client_id":   {clientID},
	})
	if err != nil {
		return nil, false, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var tr tokenResponse
		if err := json.Unmarshal(body, &tr); err != nil {
			return nil, false, fmt.Errorf("decode token response: %w", err)
		}
		return &tr, true, nil
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(body, &errResp)

	switch errResp.Error {
	case "authorization_pending":
		return nil, false, nil
	case "slow_down":
		return nil, false, errSlowDown
	default:
		return nil, false, fmt.Errorf("authorization failed: %s", errResp.Error)
	}
}
