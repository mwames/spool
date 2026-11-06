package scaffold

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// INVARIANT-1-8: Invariants Documented
func TestInit_GuideDocumentsInvariants(t *testing.T) {
	root := t.TempDir()
	runInit(t, root)

	data, err := os.ReadFile(filepath.Join(root, "SPOOL.md"))
	if err != nil {
		t.Fatalf("reading SPOOL.md: %v", err)
	}
	guide := string(data)

	if !strings.Contains(guide, "invariants:") {
		t.Error("SPOOL.md should show the invariants list in a .req file")
	}
	// The ID format, written with placeholders rather than numbers.
	if !regexp.MustCompile(`\S+-INV-[^\s\d]`).MatchString(guide) {
		t.Error("SPOOL.md should describe the <REQ>-<n>-INV-<n> ID format")
	}
	if !regexp.MustCompile(`\b[A-Z]+-\d+-INV-\d+\b`).MatchString(guide) {
		t.Error("SPOOL.md should give an example invariant ID")
	}
	if !regexp.MustCompile(`(?m)^\s*(//|#)\s*[A-Z]+-\d+-INV-\d+\b`).MatchString(guide) {
		t.Error("SPOOL.md should show a test annotated with an invariant ID")
	}
}
