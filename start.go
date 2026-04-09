package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const (
	exerciseMediaType = "application/vnd.odyssey.exercise+json"
	rootMediaType     = "application/vnd.odyssey.root+json"
	clusterName       = "odyssey"
)

// internalNamespaces are never deleted during cluster cleanup.
var internalNamespaces = map[string]bool{
	"kube-system":        true,
	"kube-public":        true,
	"kube-node-lease":    true,
	"default":            true,
	"local-path-storage": true,
}

// Step is a single exercise step to apply to the cluster.
type Step struct {
	Op       string   `json:"op"`
	Manifest string   `json:"manifest,omitempty"`
	Content  string   `json:"content,omitempty"`
	Args     []string `json:"args,omitempty"`
}

// KindManager manages kind clusters.
type KindManager interface {
	ClusterExists(name string) (bool, error)
	CreateCluster(name string) error
}

// RealKindManager uses exec.Command("kind", ...) to manage clusters.
type RealKindManager struct{}

func (r *RealKindManager) ClusterExists(name string) (bool, error) {
	out, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		return false, fmt.Errorf("kind get clusters: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *RealKindManager) CreateCluster(name string) error {
	cmd := exec.Command("kind", "create", "cluster", "--name", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kind create cluster %s: %w", name, err)
	}
	return nil
}

// MockKindManager is a test double for KindManager.
type MockKindManager struct {
	Exists       bool
	CreateCalled bool
	CreateErr    error
}

func (m *MockKindManager) ClusterExists(name string) (bool, error) {
	return m.Exists, nil
}

func (m *MockKindManager) CreateCluster(name string) error {
	m.CreateCalled = true
	return m.CreateErr
}

// fetchExercise GETs /api/ and follows the active_exercise link.
// Returns nil if the link is absent.
func fetchExercise(client *Client) (*ServerResponse, error) {
	data, err := client.Get("/api/", rootMediaType)
	if err != nil {
		return nil, fmt.Errorf("GET /api/: %w", err)
	}
	root, err := ParseResponse(data)
	if err != nil {
		return nil, fmt.Errorf("parse root: %w", err)
	}
	link, ok := root.Links["active_exercise"]
	if !ok {
		return nil, nil
	}
	exData, err := client.Get(link.Href, exerciseMediaType)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", link.Href, err)
	}
	ex, err := ParseResponse(exData)
	if err != nil {
		return nil, fmt.Errorf("parse exercise: %w", err)
	}
	return ex, nil
}

// exerciseID extracts the numeric id from the exercise response.
func exerciseID(ex *ServerResponse) int {
	v, ok := ex.raw["id"]
	if !ok {
		return 0
	}
	switch id := v.(type) {
	case float64:
		return int(id)
	case int:
		return id
	}
	return 0
}

// exerciseSteps extracts the steps slice from the exercise response.
func exerciseSteps(ex *ServerResponse) ([]Step, error) {
	v, ok := ex.raw["steps"]
	if !ok {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal steps: %w", err)
	}
	var steps []Step
	if err := json.Unmarshal(b, &steps); err != nil {
		return nil, fmt.Errorf("unmarshal steps: %w", err)
	}
	return steps, nil
}

// applyStep applies a single exercise step via kubectl.
func applyStep(kubectl Runner, step Step) error {
	switch step.Op {
	case "apply":
		return kubectl.Run([]string{"apply", "-f", "-"}, step.Content)
	case "kubectl":
		return kubectl.Run(step.Args, "")
	default:
		return fmt.Errorf("unknown step op: %s", step.Op)
	}
}

// cleanNamespaces deletes all non-internal namespaces from the cluster.
func cleanNamespaces(kubectl Runner, w io.Writer) error {
	out, err := kubectl.Output([]string{"get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}"})
	if err != nil {
		return fmt.Errorf("list namespaces: %w", err)
	}
	namespaces := strings.Fields(out)
	for _, ns := range namespaces {
		if internalNamespaces[ns] {
			continue
		}
		fmt.Fprintf(w, "Deleting namespace %s\n", ns)
		if err := kubectl.Run([]string{"delete", "namespace", ns}, ""); err != nil {
			return fmt.Errorf("delete namespace %s: %w", ns, err)
		}
	}
	return nil
}

// runStart is the core implementation of ody start.
func runStart(client *Client, kubectl Runner, kind KindManager, w io.Writer) error {
	ex, err := fetchExercise(client)
	if err != nil {
		return err
	}
	if ex == nil {
		fmt.Fprintln(w, "No active exercise")
		return nil
	}

	exists, err := kind.ClusterExists(clusterName)
	if err != nil {
		return fmt.Errorf("check cluster: %w", err)
	}
	if !exists {
		fmt.Fprintf(w, "Creating kind cluster %q\n", clusterName)
		if err := kind.CreateCluster(clusterName); err != nil {
			return err
		}
	}

	if err := cleanNamespaces(kubectl, w); err != nil {
		return err
	}

	steps, err := exerciseSteps(ex)
	if err != nil {
		return err
	}

	for i, step := range steps {
		if err := applyStep(kubectl, step); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Op, err)
		}
	}

	id := exerciseID(ex)
	fmt.Fprintf(w, "Exercise #%d applied — %d steps\n", id, len(steps))
	return nil
}
