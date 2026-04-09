package main

import (
	"strings"
	"testing"
)

func TestMockRunnerRecordsCalls(t *testing.T) {
	m := &MockRunner{}
	err := m.Run([]string{"apply", "-f", "-"}, "apiVersion: v1")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(m.RunCalls) != 1 {
		t.Fatalf("calls = %d, want 1", len(m.RunCalls))
	}
	call := m.RunCalls[0]
	if call.Args[0] != "apply" {
		t.Errorf("args[0] = %q", call.Args[0])
	}
	if call.Stdin != "apiVersion: v1" {
		t.Errorf("stdin = %q", call.Stdin)
	}
}

func TestMockRunnerOutputReturnsConfigured(t *testing.T) {
	m := &MockRunner{
		OutputResults: map[string]string{
			"get deployments -n exercise -o json": `{"items":[]}`,
		},
	}
	out, err := m.Output([]string{"get", "deployments", "-n", "exercise", "-o", "json"})
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	if out != `{"items":[]}` {
		t.Errorf("output = %q", out)
	}
	if len(m.OutputCalls) != 1 {
		t.Fatalf("calls = %d, want 1", len(m.OutputCalls))
	}
}

func TestMockRunnerOutputMissingKey(t *testing.T) {
	m := &MockRunner{
		OutputResults: map[string]string{},
	}
	_, err := m.Output([]string{"get", "pods"})
	if err == nil {
		t.Fatal("expected error for unconfigured output")
	}
}

func TestExecErrorMessage(t *testing.T) {
	e := &ExecError{Op: "apply", Err: "something broke"}
	msg := e.Error()
	if !strings.Contains(msg, "apply") || !strings.Contains(msg, "something broke") {
		t.Errorf("error = %q", msg)
	}
}
