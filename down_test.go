package main

import (
	"errors"
	"strings"
	"testing"
)

func TestDownConfirmYesDeletes(t *testing.T) {
	kind := &MockKindManager{Exists: true}
	var out strings.Builder
	in := strings.NewReader("y\n")

	if err := runDown(kind, &out, in); err != nil {
		t.Fatalf("runDown: %v", err)
	}
	if !kind.DeleteCalled {
		t.Error("DeleteCluster was not called")
	}
	if !strings.Contains(out.String(), `Deleted kind cluster "odyssey".`) {
		t.Errorf("missing success message: %q", out.String())
	}
}

func TestDownConfirmYesWordCaseInsensitive(t *testing.T) {
	for _, answer := range []string{"yes\n", "YES\n", "Y\n", "  y  \n"} {
		kind := &MockKindManager{Exists: true}
		var out strings.Builder
		if err := runDown(kind, &out, strings.NewReader(answer)); err != nil {
			t.Fatalf("runDown(%q): %v", answer, err)
		}
		if !kind.DeleteCalled {
			t.Errorf("answer %q: DeleteCluster not called", answer)
		}
	}
}

func TestDownAbortOnNo(t *testing.T) {
	for _, answer := range []string{"n\n", "no\n", "\n", "maybe\n"} {
		kind := &MockKindManager{Exists: true}
		var out strings.Builder
		if err := runDown(kind, &out, strings.NewReader(answer)); err != nil {
			t.Fatalf("runDown(%q): %v", answer, err)
		}
		if kind.DeleteCalled {
			t.Errorf("answer %q: DeleteCluster called but should not have been", answer)
		}
		if !strings.Contains(out.String(), "Aborted.") {
			t.Errorf("answer %q: missing 'Aborted.' in output: %q", answer, out.String())
		}
	}
}

func TestDownMissingClusterIsIdempotent(t *testing.T) {
	kind := &MockKindManager{Exists: false}
	var out strings.Builder
	// Reader is unused when the cluster is absent; pass an empty one to prove
	// we don't block waiting for input.
	in := strings.NewReader("")

	if err := runDown(kind, &out, in); err != nil {
		t.Fatalf("runDown: %v", err)
	}
	if kind.DeleteCalled {
		t.Error("DeleteCluster must not be called when cluster is absent")
	}
	if !strings.Contains(out.String(), `No kind cluster "odyssey" to tear down.`) {
		t.Errorf("missing informative message: %q", out.String())
	}
}

func TestDownClusterExistsError(t *testing.T) {
	kind := &MockKindManager{ExistsErr: errors.New("kind not installed")}
	var out strings.Builder
	if err := runDown(kind, &out, strings.NewReader("")); err == nil {
		t.Fatal("expected error, got nil")
	} else if !strings.Contains(err.Error(), "check cluster") {
		t.Errorf("error = %q, want 'check cluster' prefix", err.Error())
	}
}

func TestDownDeleteError(t *testing.T) {
	kind := &MockKindManager{Exists: true, DeleteErr: errors.New("boom")}
	var out strings.Builder
	err := runDown(kind, &out, strings.NewReader("y\n"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "delete cluster") {
		t.Errorf("error = %q, want 'delete cluster' prefix", err.Error())
	}
}
