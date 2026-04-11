package main

import (
	"fmt"
	"io"
)

// parseProbes extracts the probes list from the exercise response.
// Returns nil if the field is absent or not a list.
func parseProbes(ex *ServerResponse) []struct{ Pod, Namespace string } {
	v, ok := ex.Field("probes")
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var probes []struct{ Pod, Namespace string }
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pod, _ := m["pod"].(string)
		ns, _ := m["namespace"].(string)
		if pod != "" && ns != "" {
			probes = append(probes, struct{ Pod, Namespace string }{pod, ns})
		}
	}
	return probes
}

// runClue fetches the active exercise and prints probe logs.
func runClue(client *Client, kubectl Runner, w io.Writer) error {
	ex, err := fetchExercise(client)
	if err != nil {
		return err
	}
	if ex == nil {
		fmt.Fprintln(w, "No active exercise — generate one in the browser.")
		return nil
	}

	probes := parseProbes(ex)
	if len(probes) == 0 {
		fmt.Fprintln(w, "No clues available for this exercise")
		return nil
	}

	for i, probe := range probes {
		if i > 0 {
			fmt.Fprintln(w)
		}
		cmd := fmt.Sprintf("kubectl logs %s -n %s --tail=10", probe.Pod, probe.Namespace)
		fmt.Fprintf(w, "$ %s\n", cmd)
		out, err := kubectl.Output([]string{"logs", probe.Pod, "-n", probe.Namespace, "--tail=10"})
		if err != nil {
			fmt.Fprintf(w, "Error: %v\n", err)
			continue
		}
		fmt.Fprint(w, out)
	}
	return nil
}
