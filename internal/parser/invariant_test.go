package parser

import (
	"path/filepath"
	"strings"
	"testing"
)

const invariantReqYAML = `
feature: SLICE

requirements:
  - id: SLICE-6
    title: Jobs
    description: Jobs run.
    status: active
    acceptance_criteria:
      - id: SLICE-6-1
        title: Runs
        description: A job runs.
    invariants:
      - id: slice-06-inv-01
        title: No Lost Jobs
        description: A job is never dropped.
      - id: SLICE-6-INV-2
        title: Ordered
        description: Jobs finish in order.
`

// INVARIANT-1-2: Invariants Parsed
// SPEC-4-1
func TestParseFile_Invariants(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slice.req")
	writeReqFile(t, path, invariantReqYAML)

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rf.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", rf.Warnings)
	}
	if len(rf.Requirements) != 1 {
		t.Fatalf("len(Requirements) = %d, want 1", len(rf.Requirements))
	}
	r := rf.Requirements[0]

	if len(r.AcceptanceCriteria) != 1 || r.AcceptanceCriteria[0].ID != "SLICE-6-1" {
		t.Errorf("ACs = %+v, want SLICE-6-1", r.AcceptanceCriteria)
	}
	if len(r.Invariants) != 2 {
		t.Fatalf("len(Invariants) = %d, want 2", len(r.Invariants))
	}

	want := []Invariant{
		{ID: "SLICE-6-INV-1", Title: "No Lost Jobs", Description: "A job is never dropped.", Status: "active", Line: 14},
		{ID: "SLICE-6-INV-2", Title: "Ordered", Description: "Jobs finish in order.", Status: "active", Line: 17},
	}
	for i, w := range want {
		if r.Invariants[i] != w {
			t.Errorf("Invariants[%d] = %+v, want %+v", i, r.Invariants[i], w)
		}
	}

	// The ACs' line numbers are unaffected by the invariants list.
	if r.AcceptanceCriteria[0].Line != 10 {
		t.Errorf("AC[0].Line = %d, want 10", r.AcceptanceCriteria[0].Line)
	}
}

// INVARIANT-1-2: Invariants Parsed
func TestParseFile_MalformedInvariantID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.req")
	writeReqFile(t, path, `
feature: SLICE
requirements:
  - id: SLICE-6
    title: Jobs
    description: Jobs run.
    status: active
    acceptance_criteria: []
    invariants:
      - id: SLICE-6-I1
        title: Bad
        description: Bad ID.
      - id: slice-6-inv-2
        title: Good
        description: Good ID.
`)

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rf.Requirements) != 1 || len(rf.Requirements[0].Invariants) != 2 {
		t.Fatalf("Requirements = %+v, want one requirement with two invariants", rf.Requirements)
	}
	if got := rf.Requirements[0].Invariants[1].ID; got != "SLICE-6-INV-2" {
		t.Errorf("Invariants[1].ID = %q, want %q", got, "SLICE-6-INV-2")
	}

	found := false
	for _, w := range rf.Warnings {
		if strings.Contains(w, "SLICE-6-I1") {
			found = true
		}
	}
	if !found {
		t.Errorf("Warnings = %v, want one naming SLICE-6-I1", rf.Warnings)
	}
}
