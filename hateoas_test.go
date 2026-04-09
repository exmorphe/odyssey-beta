package main

import (
	"encoding/json"
	"testing"
)

func TestParseLinkFromJSON(t *testing.T) {
	raw := `{
		"href": "/exercise/42/",
		"method": "GET",
		"title": "Active exercise"
	}`
	var link Link
	if err := json.Unmarshal([]byte(raw), &link); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if link.Href != "/exercise/42/" {
		t.Errorf("href = %q", link.Href)
	}
	if link.Method != "GET" {
		t.Errorf("method = %q", link.Method)
	}
	if link.Title != "Active exercise" {
		t.Errorf("title = %q", link.Title)
	}
}

func TestParseResponseExtractsLinks(t *testing.T) {
	raw := `{
		"_type": "root",
		"_links": {
			"active_exercise": {
				"href": "/exercise/42/",
				"method": "GET",
				"title": "Active exercise"
			}
		}
	}`
	resp, err := ParseResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.Type != "root" {
		t.Errorf("type = %q", resp.Type)
	}
	if len(resp.Links) != 1 {
		t.Fatalf("links count = %d, want 1", len(resp.Links))
	}
	link, ok := resp.Links["active_exercise"]
	if !ok {
		t.Fatal("missing active_exercise link")
	}
	if link.Href != "/exercise/42/" {
		t.Errorf("href = %q", link.Href)
	}
}

func TestParseResponsePreservesRawFields(t *testing.T) {
	raw := `{
		"_type": "exercise",
		"id": 42,
		"status": "active",
		"steps": [{"op": "apply"}],
		"_links": {}
	}`
	resp, err := ParseResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.Type != "exercise" {
		t.Errorf("type = %q", resp.Type)
	}
	status, ok := resp.Field("status")
	if !ok {
		t.Fatal("missing status field")
	}
	if status != "active" {
		t.Errorf("status = %v", status)
	}
}

func TestParseResponseEmptyLinks(t *testing.T) {
	raw := `{"_type": "root", "_links": {}}`
	resp, err := ParseResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Links) != 0 {
		t.Errorf("links count = %d, want 0", len(resp.Links))
	}
}
