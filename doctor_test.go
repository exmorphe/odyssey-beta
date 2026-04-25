package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// MockProbe records calls and returns canned results. Tests only — kept in a
// _test.go file so it is not compiled into the production binary.
type MockProbe struct {
	Paths         map[string]string // bin -> path; missing key means LookPath errors
	RunResults    map[string]MockRunResult
	OS            string // empty defaults to "linux"
	GroupNames    []string
	GroupsErr     error
	LookPathCalls []string
	RunCalls      []MockRunCall
}

type MockRunResult struct {
	Stdout string
	Err    error
}

type MockRunCall struct {
	Name string
	Args []string
}

func (m *MockProbe) LookPath(bin string) (string, error) {
	m.LookPathCalls = append(m.LookPathCalls, bin)
	if path, ok := m.Paths[bin]; ok {
		return path, nil
	}
	return "", &exec.Error{Name: bin, Err: exec.ErrNotFound}
}

func (m *MockProbe) Run(name string, args ...string) (string, error) {
	m.RunCalls = append(m.RunCalls, MockRunCall{Name: name, Args: args})
	key := name
	if len(args) > 0 {
		key += " " + strings.Join(args, " ")
	}
	r, ok := m.RunResults[key]
	if !ok {
		return "", fmt.Errorf("MockProbe: no Run result configured for %q", key)
	}
	return r.Stdout, r.Err
}

func (m *MockProbe) GOOS() string {
	if m.OS == "" {
		return "linux"
	}
	return m.OS
}

func (m *MockProbe) Groups() ([]string, error) {
	return m.GroupNames, m.GroupsErr
}

func TestMockProbe_LookPathRecordsAndReturns(t *testing.T) {
	p := &MockProbe{
		Paths: map[string]string{"kind": "/usr/local/bin/kind"},
	}
	got, err := p.LookPath("kind")
	if err != nil {
		t.Fatalf("LookPath: %v", err)
	}
	if got != "/usr/local/bin/kind" {
		t.Fatalf("got %q want /usr/local/bin/kind", got)
	}
	if len(p.LookPathCalls) != 1 || p.LookPathCalls[0] != "kind" {
		t.Fatalf("LookPathCalls=%v", p.LookPathCalls)
	}
}

func TestMockProbe_LookPathMissing(t *testing.T) {
	p := &MockProbe{}
	_, err := p.LookPath("kind")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestMockProbe_RunRecordsAndReturns(t *testing.T) {
	p := &MockProbe{
		RunResults: map[string]MockRunResult{
			"kind version": {Stdout: "kind v0.22.0 go1.22 linux/amd64\n"},
		},
	}
	out, err := p.Run("kind", "version")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "kind v0.22.0 go1.22 linux/amd64\n" {
		t.Fatalf("unexpected stdout: %q", out)
	}
	if len(p.RunCalls) != 1 {
		t.Fatalf("RunCalls=%v", p.RunCalls)
	}
}

func TestMockProbe_RunErrorPropagates(t *testing.T) {
	p := &MockProbe{
		RunResults: map[string]MockRunResult{
			"docker info -f json": {Err: errors.New("Cannot connect to the Docker daemon")},
		},
	}
	_, err := p.Run("docker", "info", "-f", "json")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRealProbe_GOOSMatchesRuntime(t *testing.T) {
	p := &RealProbe{}
	if p.GOOS() != runtime.GOOS {
		t.Fatalf("GOOS=%q runtime.GOOS=%q", p.GOOS(), runtime.GOOS)
	}
}

// ---- Task 2: checkKind + checkKubectl ----

func TestCheckKind_OK(t *testing.T) {
	p := &MockProbe{
		Paths: map[string]string{"kind": "/usr/local/bin/kind"},
		RunResults: map[string]MockRunResult{
			"kind version": {Stdout: "kind v0.22.0 go1.22 linux/amd64\n"},
		},
	}
	r := checkKind(p)
	if !r.OK {
		t.Fatalf("expected OK, got %+v", r)
	}
	if !strings.Contains(r.Detail, "v0.22.0") {
		t.Fatalf("Detail=%q want it to contain v0.22.0", r.Detail)
	}
	if r.Hint != "" {
		t.Fatalf("Hint=%q want empty", r.Hint)
	}
}

func TestCheckKind_Missing(t *testing.T) {
	p := &MockProbe{}
	r := checkKind(p)
	if r.OK {
		t.Fatal("expected not OK")
	}
	if r.Hint == "" || !strings.Contains(r.Hint, "kind.sigs.k8s.io") {
		t.Fatalf("Hint=%q want install link", r.Hint)
	}
}

func TestCheckKind_VersionFails(t *testing.T) {
	p := &MockProbe{
		Paths: map[string]string{"kind": "/usr/local/bin/kind"},
		RunResults: map[string]MockRunResult{
			"kind version": {Err: errors.New("exit 1")},
		},
	}
	r := checkKind(p)
	if r.OK {
		t.Fatal("expected not OK when version fails")
	}
}

func TestCheckKubectl_OK(t *testing.T) {
	p := &MockProbe{
		Paths: map[string]string{"kubectl": "/usr/local/bin/kubectl"},
		RunResults: map[string]MockRunResult{
			"kubectl version --client -o json": {
				Stdout: `{"clientVersion":{"gitVersion":"v1.29.1"}}`,
			},
		},
	}
	r := checkKubectl(p)
	if !r.OK {
		t.Fatalf("expected OK, got %+v", r)
	}
	if !strings.Contains(r.Detail, "v1.29.1") {
		t.Fatalf("Detail=%q want v1.29.1", r.Detail)
	}
}

func TestCheckKubectl_Missing(t *testing.T) {
	p := &MockProbe{}
	r := checkKubectl(p)
	if r.OK {
		t.Fatal("expected not OK")
	}
	if !strings.Contains(r.Hint, "kubernetes.io") {
		t.Fatalf("Hint=%q want install link", r.Hint)
	}
}

func TestCheckKubectl_VersionFails_StillOKIfClientPresent(t *testing.T) {
	// kubectl version --client can fail to parse on weird builds; we want a soft pass:
	// if LookPath worked and the binary ran (even with garbage output), report OK with a generic detail.
	p := &MockProbe{
		Paths: map[string]string{"kubectl": "/usr/local/bin/kubectl"},
		RunResults: map[string]MockRunResult{
			"kubectl version --client -o json": {Stdout: "not json"},
		},
	}
	r := checkKubectl(p)
	if !r.OK {
		t.Fatalf("expected OK with degraded detail, got %+v", r)
	}
}

// ---- Task 3: checkDocker + checkMemory ----

func TestCheckDocker_OK(t *testing.T) {
	p := &MockProbe{
		Paths: map[string]string{"docker": "/usr/local/bin/docker"},
		RunResults: map[string]MockRunResult{
			"docker info -f {{json .}}": {
				Stdout: `{"ServerVersion":"25.0.3","MemTotal":4294967296}`,
			},
		},
	}
	r, info := checkDocker(p)
	if !r.OK {
		t.Fatalf("expected OK, got %+v", r)
	}
	if info == nil || info.ServerVersion != "25.0.3" {
		t.Fatalf("info=%+v", info)
	}
	if info.MemTotal != 4294967296 {
		t.Fatalf("MemTotal=%d", info.MemTotal)
	}
}

func TestCheckDocker_Missing(t *testing.T) {
	p := &MockProbe{}
	r, info := checkDocker(p)
	if r.OK {
		t.Fatal("expected not OK")
	}
	if info != nil {
		t.Fatal("expected nil info on missing docker")
	}
	if !strings.Contains(r.Hint, "docker.com") {
		t.Fatalf("Hint=%q want install link", r.Hint)
	}
}

func TestCheckDocker_DaemonDown(t *testing.T) {
	p := &MockProbe{
		Paths: map[string]string{"docker": "/usr/local/bin/docker"},
		RunResults: map[string]MockRunResult{
			"docker info -f {{json .}}": {Err: errors.New("Cannot connect to the Docker daemon")},
		},
	}
	r, info := checkDocker(p)
	if r.OK {
		t.Fatal("expected not OK")
	}
	if info != nil {
		t.Fatal("expected nil info when daemon is down")
	}
	if !strings.Contains(r.Hint, "daemon") {
		t.Fatalf("Hint=%q want daemon hint", r.Hint)
	}
}

func TestCheckMemory_OK(t *testing.T) {
	info := &dockerInfo{MemTotal: 4 * 1024 * 1024 * 1024}
	r := checkMemory(info)
	if !r.OK {
		t.Fatalf("expected OK, got %+v", r)
	}
	if !strings.Contains(r.Detail, "GiB") {
		t.Fatalf("Detail=%q want GiB", r.Detail)
	}
}

func TestCheckMemory_BelowFloor(t *testing.T) {
	info := &dockerInfo{MemTotal: 1500 * 1024 * 1024} // 1.5 GiB
	r := checkMemory(info)
	if r.OK {
		t.Fatal("expected not OK below floor")
	}
	if !strings.Contains(r.Hint, "2 GiB") {
		t.Fatalf("Hint=%q want '2 GiB' threshold mentioned", r.Hint)
	}
}

func TestCheckMemory_NilInfo(t *testing.T) {
	// When docker info is unavailable we soft-skip: docker has already reported
	// its own ✗ with a remediation hint; flagging memory would be redundant noise.
	r := checkMemory(nil)
	if !r.OK {
		t.Fatalf("expected OK (soft skip) when info nil, got %+v", r)
	}
	if !strings.Contains(strings.ToLower(r.Detail), "skip") {
		t.Fatalf("Detail=%q want 'skip' indication", r.Detail)
	}
	if r.Hint != "" {
		t.Fatalf("Hint=%q want empty on soft skip", r.Hint)
	}
}

// TestRunDoctor_DockerDownDoesNotDoubleFail asserts that when docker fails,
// memory is reported as a soft skip (no second ✗) so the user sees one
// actionable failure, not two.
func TestRunDoctor_DockerDownDoesNotDoubleFail(t *testing.T) {
	p := &MockProbe{
		Paths: map[string]string{
			"docker":  "/usr/local/bin/docker",
			"kind":    "/usr/local/bin/kind",
			"kubectl": "/usr/local/bin/kubectl",
		},
		RunResults: map[string]MockRunResult{
			"docker info -f {{json .}}":        {Err: errors.New("Cannot connect to the Docker daemon")},
			"kind version":                     {Stdout: "kind v0.22.0\n"},
			"kubectl version --client -o json": {Stdout: `{"clientVersion":{"gitVersion":"v1.29.1"}}`},
		},
		OS:         "linux",
		GroupNames: []string{"docker"},
	}
	var buf bytes.Buffer
	code := runDoctor(p, &buf)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	out := buf.String()
	// Exactly one ✗ — on docker. Memory should be ✓ (soft skip).
	if got := strings.Count(out, "✗"); got != 1 {
		t.Errorf("expected exactly 1 ✗ in output, got %d:\n%s", got, out)
	}
}

// ---- Task 4: checkDockerGroup ----

func TestCheckDockerGroup_LinuxInGroup(t *testing.T) {
	p := &MockProbe{OS: "linux", GroupNames: []string{"users", "docker"}}
	r := checkDockerGroup(p)
	if !r.OK {
		t.Fatalf("expected OK, got %+v", r)
	}
}

func TestCheckDockerGroup_LinuxNotInGroup(t *testing.T) {
	p := &MockProbe{OS: "linux", GroupNames: []string{"users"}}
	r := checkDockerGroup(p)
	if r.OK {
		t.Fatal("expected not OK")
	}
	if !strings.Contains(r.Hint, "usermod") {
		t.Fatalf("Hint=%q want usermod mention", r.Hint)
	}
}

func TestCheckDockerGroup_LinuxGroupsError(t *testing.T) {
	p := &MockProbe{OS: "linux", GroupsErr: errors.New("nss boom")}
	r := checkDockerGroup(p)
	if r.OK {
		t.Fatal("expected not OK on groups error")
	}
}

func TestCheckDockerGroup_Darwin(t *testing.T) {
	p := &MockProbe{OS: "darwin"}
	r := checkDockerGroup(p)
	if !r.OK {
		t.Fatalf("expected OK (skipped) on darwin, got %+v", r)
	}
	if !strings.Contains(strings.ToLower(r.Detail), "skip") {
		t.Fatalf("Detail=%q want 'skipped' indication", r.Detail)
	}
}

// ---- Task 5: runChecks + runDoctor ----

// allOKProbe builds a MockProbe where every check passes.
func allOKProbe() *MockProbe {
	return &MockProbe{
		Paths: map[string]string{
			"docker":  "/usr/local/bin/docker",
			"kind":    "/usr/local/bin/kind",
			"kubectl": "/usr/local/bin/kubectl",
		},
		RunResults: map[string]MockRunResult{
			"docker info -f {{json .}}":        {Stdout: `{"ServerVersion":"25.0.3","MemTotal":4294967296}`},
			"kind version":                     {Stdout: "kind v0.22.0\n"},
			"kubectl version --client -o json": {Stdout: `{"clientVersion":{"gitVersion":"v1.29.1"}}`},
		},
		OS:         "linux",
		GroupNames: []string{"docker"},
	}
}

func TestRunChecks_Order(t *testing.T) {
	p := allOKProbe()
	results := runChecks(p)
	want := []string{"docker", "kind", "kubectl", "group", "memory"}
	if len(results) != len(want) {
		t.Fatalf("len=%d want %d", len(results), len(want))
	}
	for i, name := range want {
		if results[i].Name != name {
			t.Errorf("results[%d].Name=%q want %q", i, results[i].Name, name)
		}
	}
}

func TestRunDoctor_AllPass(t *testing.T) {
	var buf bytes.Buffer
	code := runDoctor(allOKProbe(), &buf)
	if code != 0 {
		t.Fatalf("exit=%d want 0", code)
	}
	out := buf.String()
	for _, name := range []string{"docker", "kind", "kubectl", "group", "memory"} {
		if !strings.Contains(out, name) {
			t.Errorf("output missing %q:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "All checks passed") {
		t.Errorf("missing summary in:\n%s", out)
	}
}

func TestRunDoctor_FailureExitCodeAndHints(t *testing.T) {
	p := &MockProbe{
		// docker missing
		Paths: map[string]string{
			"kind":    "/usr/local/bin/kind",
			"kubectl": "/usr/local/bin/kubectl",
		},
		RunResults: map[string]MockRunResult{
			"kind version":                     {Stdout: "kind v0.22.0\n"},
			"kubectl version --client -o json": {Stdout: `{"clientVersion":{"gitVersion":"v1.29.1"}}`},
		},
		OS:         "linux",
		GroupNames: []string{"docker"},
	}
	var buf bytes.Buffer
	code := runDoctor(p, &buf)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	if !strings.Contains(buf.String(), "✗") {
		t.Errorf("missing ✗ glyph in:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "docker.com") {
		t.Errorf("missing docker install hint in:\n%s", buf.String())
	}
}

// ---- Task 6: preflight ----

func TestPreflight_AllPassOneLine(t *testing.T) {
	var buf bytes.Buffer
	err := preflight(allOKProbe(), &buf)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	out := buf.String()
	lines := strings.Count(out, "\n")
	if lines != 1 {
		t.Errorf("expected 1 line, got %d:\n%s", lines, out)
	}
	if !strings.HasPrefix(out, "Preflight:") {
		t.Errorf("output should start with 'Preflight:': %q", out)
	}
	for _, name := range []string{"docker", "kind", "kubectl", "memory"} {
		if !strings.Contains(out, name) {
			t.Errorf("missing %q", name)
		}
	}
}

func TestPreflight_FailureBlock(t *testing.T) {
	p := &MockProbe{
		// nothing in Paths -> all binary checks fail
		OS:         "linux",
		GroupNames: []string{"docker"},
	}
	var buf bytes.Buffer
	err := preflight(p, &buf)
	if err == nil {
		t.Fatal("expected error on failed preflight")
	}
	out := buf.String()
	if !strings.Contains(out, "Preflight failed") {
		t.Errorf("missing 'Preflight failed' header:\n%s", out)
	}
	if !strings.Contains(out, "ody doctor") {
		t.Errorf("missing 'ody doctor' pointer:\n%s", out)
	}
}
