package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

// Probe is the system-level surface the doctor checks need. Mocked in tests.
type Probe interface {
	// LookPath returns the absolute path of bin if found on $PATH, error otherwise.
	LookPath(bin string) (string, error)
	// Run executes name with args and returns combined stdout. Stderr is captured
	// and folded into the returned error on non-zero exit.
	Run(name string, args ...string) (string, error)
	// GOOS returns the running OS — "linux", "darwin", etc.
	GOOS() string
	// Groups returns the current user's group names.
	Groups() ([]string, error)
}

// CheckResult is the outcome of a single check.
type CheckResult struct {
	Name   string // short label e.g. "docker", "kind"
	OK     bool
	Detail string // one-line summary shown next to the name
	Hint   string // remediation hint shown when OK is false; empty when OK
}

// RealProbe is the production Probe implementation.
type RealProbe struct{}

func (RealProbe) LookPath(bin string) (string, error) {
	return exec.LookPath(bin)
}

func (RealProbe) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr != "" {
			return string(out), fmt.Errorf("%s: %w (%s)", name, err, stderr)
		}
		return string(out), fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}

func (RealProbe) GOOS() string { return runtime.GOOS }

func (RealProbe) Groups() ([]string, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	gids, err := u.GroupIds()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(gids))
	for _, gid := range gids {
		g, err := user.LookupGroupId(gid)
		if err != nil {
			continue
		}
		names = append(names, g.Name)
	}
	return names, nil
}

// ---- check helpers ----

const (
	kindInstallURL    = "https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
	kubectlInstallURL = "https://kubernetes.io/docs/tasks/tools/"
	dockerInstallURL  = "https://docs.docker.com/engine/install/"
	memoryFloor       = int64(2) * 1024 * 1024 * 1024 // 2 GiB
)

func checkKind(p Probe) CheckResult {
	r := CheckResult{Name: "kind"}
	if _, err := p.LookPath("kind"); err != nil {
		r.Detail = "not found on PATH"
		r.Hint = "install kind: " + kindInstallURL
		return r
	}
	out, err := p.Run("kind", "version")
	if err != nil {
		r.Detail = "kind binary present but `kind version` failed"
		r.Hint = "reinstall kind: " + kindInstallURL
		return r
	}
	r.OK = true
	r.Detail = firstWordAfterPrefix(out, "kind ")
	if r.Detail == "" {
		r.Detail = "installed"
	}
	return r
}

func checkKubectl(p Probe) CheckResult {
	r := CheckResult{Name: "kubectl"}
	if _, err := p.LookPath("kubectl"); err != nil {
		r.Detail = "not found on PATH"
		r.Hint = "install kubectl: " + kubectlInstallURL
		return r
	}
	out, err := p.Run("kubectl", "version", "--client", "-o", "json")
	if err != nil {
		// Binary present, version subcmd unhappy — degraded but acceptable.
		r.OK = true
		r.Detail = "installed (version probe failed)"
		return r
	}
	var parsed struct {
		ClientVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"clientVersion"`
	}
	r.OK = true
	if err := json.Unmarshal([]byte(out), &parsed); err == nil && parsed.ClientVersion.GitVersion != "" {
		r.Detail = parsed.ClientVersion.GitVersion + " (client)"
	} else {
		r.Detail = "installed"
	}
	return r
}

// firstWordAfterPrefix returns the first whitespace-delimited token after prefix
// in s. Used to pull "v0.22.0" out of "kind v0.22.0 go1.22 linux/amd64".
func firstWordAfterPrefix(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	fields := strings.Fields(s[idx+len(prefix):])
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// dockerInfo is the subset of `docker info -f '{{json .}}'` we parse.
type dockerInfo struct {
	ServerVersion string `json:"ServerVersion"`
	MemTotal      int64  `json:"MemTotal"` // bytes; reflects Docker Desktop / OrbStack VM on macOS
}

func checkDocker(p Probe) (CheckResult, *dockerInfo) {
	r := CheckResult{Name: "docker"}
	if _, err := p.LookPath("docker"); err != nil {
		r.Detail = "not found on PATH"
		r.Hint = "install Docker (or OrbStack on macOS): " + dockerInstallURL
		return r, nil
	}
	out, err := p.Run("docker", "info", "-f", "{{json .}}")
	if err != nil {
		r.Detail = "daemon not reachable"
		r.Hint = "start the Docker daemon (Docker Desktop / OrbStack), then re-run"
		return r, nil
	}
	var info dockerInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		r.Detail = "could not parse `docker info` output"
		r.Hint = "ensure your Docker version is recent: " + dockerInstallURL
		return r, nil
	}
	r.OK = true
	if info.ServerVersion != "" {
		r.Detail = "Docker " + info.ServerVersion
	} else {
		r.Detail = "daemon reachable"
	}
	return r, &info
}

func checkMemory(info *dockerInfo) CheckResult {
	r := CheckResult{Name: "memory"}
	if info == nil {
		// Soft skip: docker already reported its own ✗ with a hint;
		// surfacing a second ✗ here would just be noise.
		r.OK = true
		r.Detail = "skipped (docker check failed)"
		return r
	}
	gib := float64(info.MemTotal) / (1024 * 1024 * 1024)
	if info.MemTotal < memoryFloor {
		r.Detail = fmt.Sprintf("Docker VM has %.1f GiB allocated", gib)
		r.Hint = "kind needs ~2 GiB; raise Docker Desktop / OrbStack memory limit"
		return r
	}
	r.OK = true
	r.Detail = fmt.Sprintf("%.1f GiB allocated to Docker", gib)
	return r
}

func checkDockerGroup(p Probe) CheckResult {
	r := CheckResult{Name: "group"}
	if p.GOOS() != "linux" {
		r.OK = true
		r.Detail = "skipped (not linux)"
		return r
	}
	groups, err := p.Groups()
	if err != nil {
		r.Detail = "could not read user groups: " + err.Error()
		return r
	}
	for _, g := range groups {
		if g == "docker" {
			r.OK = true
			r.Detail = "user is in 'docker' group"
			return r
		}
	}
	r.Detail = "user is not in 'docker' group"
	r.Hint = "sudo usermod -aG docker $USER, then log out and back in"
	return r
}

// runChecks runs all preflight checks in stable order: docker, kind, kubectl, group, memory.
// Memory comes last because it depends on docker info from the docker check.
func runChecks(p Probe) []CheckResult {
	dockerR, info := checkDocker(p)
	return []CheckResult{
		dockerR,
		checkKind(p),
		checkKubectl(p),
		checkDockerGroup(p),
		checkMemory(info),
	}
}

// runDoctor prints a verbose report of every check and returns 0 / 1 for the process exit code.
func runDoctor(p Probe, w io.Writer) int {
	results := runChecks(p)
	allOK := true
	for _, r := range results {
		glyph := "✓"
		if !r.OK {
			glyph = "✗"
			allOK = false
		}
		fmt.Fprintf(w, "%-10s %s  %s\n", r.Name, glyph, r.Detail)
	}
	if allOK {
		fmt.Fprintln(w, "\nAll checks passed. Ready for 'ody start'.")
		return 0
	}
	fmt.Fprintln(w)
	for _, r := range results {
		if !r.OK && r.Hint != "" {
			fmt.Fprintf(w, "  %s: %s\n", r.Name, r.Hint)
		}
	}
	fmt.Fprintln(w, "\nFix the above and re-run.")
	return 1
}

// preflight runs the doctor checks and prints a compact summary suitable for
// being prepended to `ody start` output. Returns an error when any check fails.
func preflight(p Probe, w io.Writer) error {
	results := runChecks(p)
	allOK := true
	var parts []string
	for _, r := range results {
		glyph := "✓"
		if !r.OK {
			glyph = "✗"
			allOK = false
		}
		parts = append(parts, fmt.Sprintf("%s %s", r.Name, glyph))
	}
	if allOK {
		fmt.Fprintln(w, "Preflight: "+strings.Join(parts, "  "))
		return nil
	}
	fmt.Fprintln(w, "Preflight failed:")
	for _, r := range results {
		glyph := "✓"
		if !r.OK {
			glyph = "✗"
		}
		line := fmt.Sprintf("  %s %s", glyph, r.Name)
		if r.Detail != "" {
			line += " — " + r.Detail
		}
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run 'ody doctor' for details.")
	return fmt.Errorf("preflight checks failed")
}
