package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion_Default(t *testing.T) {
	var buf bytes.Buffer
	runVersion(&buf)
	got := buf.String()
	if !strings.HasPrefix(got, "ody dev (none, built unknown)") {
		t.Fatalf("unexpected default output: %q", got)
	}
}

func TestRunVersion_Injected(t *testing.T) {
	t.Cleanup(func() { version, commit, date = "dev", "none", "unknown" })
	version, commit, date = "0.1.0", "abc1234", "2026-04-16T12:00:00Z"
	var buf bytes.Buffer
	runVersion(&buf)
	want := "ody 0.1.0 (abc1234, built 2026-04-16T12:00:00Z)\n"
	if buf.String() != want {
		t.Fatalf("got %q want %q", buf.String(), want)
	}
}
