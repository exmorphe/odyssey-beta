package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes kubectl commands.
type Runner interface {
	Run(args []string, stdin string) error
	Output(args []string) (string, error)
}

// ExecError is a kubectl execution error.
type ExecError struct {
	Op  string
	Err string
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("kubectl %s: %s", e.Op, e.Err)
}

// KubectlRunner executes kubectl commands as subprocesses.
type KubectlRunner struct{}

func (k *KubectlRunner) Run(args []string, stdin string) error {
	cmd := exec.Command("kubectl", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		op := "exec"
		if len(args) > 0 {
			op = args[0]
		}
		return &ExecError{Op: op, Err: fmt.Sprintf("%v: %s", err, stderr.String())}
	}
	return nil
}

func (k *KubectlRunner) Output(args []string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		op := "exec"
		if len(args) > 0 {
			op = args[0]
		}
		return "", &ExecError{Op: op, Err: fmt.Sprintf("%v: %s", err, stderr.String())}
	}
	return stdout.String(), nil
}

// RunCall records a single Run invocation for MockRunner.
type RunCall struct {
	Args  []string
	Stdin string
}

// MockRunner records kubectl calls and returns configurable results.
type MockRunner struct {
	RunCalls      []RunCall
	RunErr        error
	OutputCalls   [][]string
	OutputResults map[string]string // key: strings.Join(args, " ")
	OutputErr     error
}

func (m *MockRunner) Run(args []string, stdin string) error {
	m.RunCalls = append(m.RunCalls, RunCall{Args: args, Stdin: stdin})
	return m.RunErr
}

func (m *MockRunner) Output(args []string) (string, error) {
	m.OutputCalls = append(m.OutputCalls, args)
	if m.OutputErr != nil {
		return "", m.OutputErr
	}
	key := strings.Join(args, " ")
	result, ok := m.OutputResults[key]
	if !ok {
		return "", fmt.Errorf("no configured output for: %s", key)
	}
	return result, nil
}
