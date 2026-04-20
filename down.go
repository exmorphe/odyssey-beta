package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// runDown tears down the local kind cluster after confirming with the user.
func runDown(kind KindManager, w io.Writer, r io.Reader) error {
	exists, err := kind.ClusterExists(clusterName)
	if err != nil {
		return fmt.Errorf("check cluster: %w", err)
	}
	if !exists {
		fmt.Fprintf(w, "No kind cluster %q to tear down.\n", clusterName)
		return nil
	}

	fmt.Fprintf(w, "Delete kind cluster %q? [y/N]: ", clusterName)
	reader := bufio.NewReader(r)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(w, "Aborted.")
		return nil
	}

	if err := kind.DeleteCluster(clusterName); err != nil {
		return fmt.Errorf("delete cluster: %w", err)
	}
	fmt.Fprintf(w, "Deleted kind cluster %q.\n", clusterName)
	return nil
}
