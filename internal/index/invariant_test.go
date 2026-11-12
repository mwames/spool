package index

import (
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

func inv(id string) parser.Invariant {
	return parser.Invariant{ID: id, Title: id, Description: "test"}
}

// withInvariants returns r with the given invariants, as parsed: one with no
// status of its own takes the requirement's (SPEC-4-1).
func withInvariants(r parser.Requirement, invs ...parser.Invariant) parser.Requirement {
	for i := range invs {
		if invs[i].Status == "" {
			invs[i].Status = r.Status
		}
	}
	r.Invariants = invs
	return r
}

// INVARIANT-1-3: Invariant Annotations
func TestBuild_InvariantLinkedToTest(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("SLICE", withInvariants(req("SLICE-6", "active", ac("SLICE-6-1")), inv("SLICE-6-INV-1"))),
		},
		[]scanner.FileResult{
			scanResult("slice_test.go", mapping("TestNoLostJobs", 5, "SLICE-6-INV-1")),
		},
	)

	i := idx.Invariant("SLICE-6-INV-1")
	if i == nil {
		t.Fatal("SLICE-6-INV-1 not found")
	}
	if len(i.Tests) != 1 {
		t.Fatalf("len(Tests) = %d, want 1", len(i.Tests))
	}
	if i.Tests[0].Function.Name != "TestNoLostJobs" {
		t.Errorf("Function.Name = %q, want %q", i.Tests[0].Function.Name, "TestNoLostJobs")
	}
	if i.Tests[0].File != "slice_test.go" {
		t.Errorf("File = %q, want %q", i.Tests[0].File, "slice_test.go")
	}

	r := idx.Requirement("SLICE-6")
	if r == nil {
		t.Fatal("SLICE-6 not found")
	}
	if len(r.Invariants) != 1 || r.Invariants[0].ID != "SLICE-6-INV-1" {
		t.Errorf("Requirement.Invariants = %v, want SLICE-6-INV-1", r.Invariants)
	}
	if len(r.ACs) != 1 || r.ACs[0].ID != "SLICE-6-1" {
		t.Errorf("Requirement.ACs = %v, want SLICE-6-1", r.ACs)
	}

	if len(idx.OrphanedAnnotations()) != 0 {
		t.Errorf("Orphaned = %v, want none", idx.OrphanedAnnotations())
	}
}

// INVARIANT-1-3: Invariant Annotations
func TestBuild_UnknownInvariantOrphaned(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("SLICE", withInvariants(req("SLICE-6", "active"), inv("SLICE-6-INV-1"))),
		},
		[]scanner.FileResult{
			scanResult("slice_test.go",
				mapping("TestNoLostJobs", 5, "SLICE-6-INV-1"),
				mapping("TestUnknown", 10, "SLICE-6-INV-9"),
			),
		},
	)

	orphaned := idx.OrphanedAnnotations()
	if len(orphaned) != 1 {
		t.Fatalf("Orphaned = %v, want only SLICE-6-INV-9", orphaned)
	}
	if orphaned[0].ID != "SLICE-6-INV-9" {
		t.Errorf("ID = %q, want SLICE-6-INV-9", orphaned[0].ID)
	}
	if orphaned[0].File != "slice_test.go" {
		t.Errorf("File = %q, want slice_test.go", orphaned[0].File)
	}
	if idx.Invariant("SLICE-6-INV-9") != nil {
		t.Error("expected nil for unknown invariant")
	}
}

// INVARIANT-1-4: Duplicate Invariant ID
func TestBuild_DuplicateInvariantID(t *testing.T) {
	sameFile := reqFile("SLICE",
		withInvariants(req("SLICE-6", "active"), inv("SLICE-6-INV-1")),
		withInvariants(req("SLICE-7", "active"), inv("SLICE-6-INV-1")),
	)
	fileA := reqFile("SLICE", withInvariants(req("SLICE-6", "active"), inv("SLICE-6-INV-1")))
	fileA.SourceFile = "/path/a.req"
	fileB := reqFile("SLICE", withInvariants(req("SLICE-7", "active"), inv("SLICE-6-INV-1")))
	fileB.SourceFile = "/path/b.req"

	tests := []struct {
		name  string
		files []*parser.ReqFile
	}{
		{"same file", []*parser.ReqFile{sameFile}},
		{"different files", []*parser.ReqFile{fileA, fileB}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := Build(tt.files, nil)

			found := false
			for _, e := range idx.Errors() {
				named := strings.Contains(e.Message, "SLICE-6-INV-1")
				for _, f := range tt.files {
					named = named && strings.Contains(e.Message, f.SourceFile)
				}
				if named {
					found = true
				}
			}
			if !found {
				t.Errorf("expected error naming SLICE-6-INV-1 and both source files, got %v", idx.Errors())
			}
		})
	}

	t.Run("distinct IDs", func(t *testing.T) {
		idx := Build(
			[]*parser.ReqFile{
				reqFile("SLICE", withInvariants(req("SLICE-6", "active"), inv("SLICE-6-INV-1"), inv("SLICE-6-INV-2"))),
			},
			nil,
		)
		if len(idx.Errors()) != 0 {
			t.Errorf("expected no errors, got %v", idx.Errors())
		}
	})
}

// INVARIANT-1-5: Untested Invariants Reported
func TestUntestedInvariants(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("SLICE",
				withInvariants(req("SLICE-6", "active"), inv("SLICE-6-INV-1"), inv("SLICE-6-INV-2")),
				withInvariants(req("SLICE-7", "superseded"), inv("SLICE-7-INV-1")),
			),
		},
		[]scanner.FileResult{
			scanResult("slice_test.go", mapping("TestNoLostJobs", 5, "SLICE-6-INV-1")),
		},
	)

	untested := idx.UntestedInvariants()
	if len(untested) != 1 {
		t.Fatalf("UntestedInvariants = %v, want only SLICE-6-INV-2", untested)
	}
	if untested[0].ID != "SLICE-6-INV-2" {
		t.Errorf("ID = %q, want SLICE-6-INV-2", untested[0].ID)
	}
	if len(idx.UntestedACs()) != 0 {
		t.Errorf("UntestedACs = %v, want none", idx.UntestedACs())
	}
}
