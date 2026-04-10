package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const snapshotMediaType = "application/vnd.odyssey.cluster-snapshot+json"

// faultResult holds the verification result for a single fault.
type faultResult struct {
	FaultKey string   `json:"fault_key"`
	FaultIDs []string `json:"fault_ids"`
	Result   string   `json:"result"`
	Masking  string   `json:"masking"`
	MaskedBy *string  `json:"masked_by"`
	Symptom  string   `json:"symptom,omitempty"`
}

// verificationResponse is the server's response to a snapshot POST.
type verificationResponse struct {
	Type   string        `json:"_type"`
	Status string        `json:"status"`
	Faults []faultResult `json:"faults"`
}

// parseNamespaces extracts namespace names from apply steps whose content
// contains a kind: Namespace manifest.
func parseNamespaces(steps []Step) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range steps {
		if s.Op != "apply" {
			continue
		}
		if !containsKind(s.Content, "Namespace") {
			continue
		}
		name := extractMetadataName(s.Content)
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// parseKinds extracts distinct resource kinds from apply steps, excluding Namespace.
func parseKinds(steps []Step) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range steps {
		if s.Op != "apply" {
			continue
		}
		k := extractKind(s.Content)
		if k == "" || k == "Namespace" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// containsKind reports whether the YAML content has a top-level kind: <target> line.
// NOTE: Line-based heuristic — assumes single-doc YAML from server-generated manifests.
// Will not work for multi-doc YAML, quoted values, or inline comments.
func containsKind(content, target string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "kind:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
			if val == target {
				return true
			}
		}
	}
	return false
}

// extractKind returns the value of the first kind: line in the YAML content.
func extractKind(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "kind:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
		}
	}
	return ""
}

// extractMetadataName returns the value of the first name: line that appears
// under a metadata: section, using simple line-based heuristics.
func extractMetadataName(content string) string {
	inMetadata := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "metadata:" {
			inMetadata = true
			continue
		}
		if inMetadata {
			// A new top-level key (no leading spaces) ends metadata.
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && trimmed != "" {
				inMetadata = false
				continue
			}
			if strings.HasPrefix(trimmed, "name:") {
				return strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			}
		}
	}
	return ""
}

// parseSnapshotKinds extracts the snapshot_kinds field from the exercise response.
// Returns nil if the field is absent (backwards compat with older servers).
func parseSnapshotKinds(ex *ServerResponse) []string {
	v, ok := ex.Field("snapshot_kinds")
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// captureSnapshot runs kubectl get <kind> -n <ns> -o json for each namespace×kind
// and merges all results into a single map keyed by "<kind>/<ns>".
func captureSnapshot(kubectl Runner, namespaces, kinds []string) (map[string]any, error) {
	snapshot := map[string]any{}
	for _, ns := range namespaces {
		for _, kind := range kinds {
			args := []string{"get", kind, "-n", ns, "-o", "json"}
			out, err := kubectl.Output(args)
			if err != nil {
				return nil, fmt.Errorf("kubectl get %s -n %s: %w", kind, ns, err)
			}
			var result any
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				return nil, fmt.Errorf("parse kubectl output for %s/%s: %w", kind, ns, err)
			}
			key := kind + "/" + ns
			snapshot[key] = result
		}
	}
	return snapshot, nil
}

// runVerify is the core implementation of ody verify.
func runVerify(client *Client, kubectl Runner, w io.Writer) error {
	ex, err := fetchExercise(client)
	if err != nil {
		return err
	}
	if ex == nil {
		fmt.Fprintln(w, "No active exercise")
		return nil
	}

	steps, err := exerciseSteps(ex)
	if err != nil {
		return err
	}

	namespaces := parseNamespaces(steps)
	kinds := parseKinds(steps)

	// Merge server-declared snapshot kinds (e.g., Endpoints) with
	// manifest-derived kinds. The server knows which extra resource
	// types verification needs; the CLI captures them without needing
	// to understand why.
	if extra := parseSnapshotKinds(ex); len(extra) > 0 {
		seen := map[string]bool{}
		for _, k := range kinds {
			seen[k] = true
		}
		for _, k := range extra {
			if !seen[k] {
				kinds = append(kinds, k)
				seen[k] = true
			}
		}
	}

	snapshot, err := captureSnapshot(kubectl, namespaces, kinds)
	if err != nil {
		return fmt.Errorf("capture snapshot: %w", err)
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	selfLink, ok := ex.Links["self"]
	if !ok {
		return fmt.Errorf("exercise response missing self link")
	}

	respData, err := client.Post(selfLink.Href, snapshotMediaType, payload)
	if err != nil {
		return fmt.Errorf("POST snapshot: %w", err)
	}

	var vr verificationResponse
	if err := json.Unmarshal(respData, &vr); err != nil {
		return fmt.Errorf("parse verification response: %w", err)
	}

	displayFaultResults(w, vr)
	return nil
}

// displayFaultResults prints per-fault results and a summary line.
func displayFaultResults(w io.Writer, vr verificationResponse) {
	// Build a map of fault_key → result so we can check blocker status.
	resultMap := make(map[string]string, len(vr.Faults))
	for _, f := range vr.Faults {
		resultMap[f.FaultKey] = f.Result
	}

	passed := 0
	for _, f := range vr.Faults {
		if f.Result == "PASS" {
			passed++
			fmt.Fprintf(w, "✓ %s — PASS\n", f.FaultKey)
		} else {
			line := fmt.Sprintf("✗ %s — FAIL", f.FaultKey)
			// Only show masking when the blocker is still failing.
			showMasking := f.Masking == "masked" && f.MaskedBy != nil && resultMap[*f.MaskedBy] != "PASS"
			if showMasking {
				line += fmt.Sprintf(" (masked by %s)", *f.MaskedBy)
			}
			fmt.Fprintln(w, line)
			if f.Symptom != "" {
				symptom := f.Symptom
				if showMasking {
					symptom += fmt.Sprintf(" — fix %s first", *f.MaskedBy)
				}
				fmt.Fprintf(w, "  symptom: %s\n", symptom)
			}
		}
	}

	total := len(vr.Faults)
	fmt.Fprintf(w, "\n%d/%d faults resolved\n", passed, total)
	if vr.Status == "solved" {
		fmt.Fprintln(w, "Solved!")
	}
}
