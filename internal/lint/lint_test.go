package lint

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
)

// SPEC-3-1
func TestGivenWhenThen(t *testing.T) {
	for _, tc := range []struct {
		description string
		want        bool
	}{
		// The spec's examples.
		{"Given a project, when spool runs, then it exits 0", true},
		{"GIVEN x\nWHEN y\nTHEN z", true},
		{"Given x, then y", false},
		{"When x, given y,\nthen z", false},
		{"Givens x, when y, then z", false},
		{"Given x, then y, when z", false},
		{"Given x when y", false},

		// Leading whitespace is ignored.
		{"  \n\tGiven x, when y, then z", true},
		// Any case.
		{"given x, when y, then z", true},
		{"gIvEn x, WhEn y, tHeN z", true},
		// Across line breaks, as a folded or literal description reads.
		{"Given a requirement,\nwhen its .req file is parsed,\nthen it holds the paths.\n", true},
		{"Given x,\n  when y,\n  then z", true},
		// Punctuation ends a word.
		{"Given: x. When: y. Then: z.", true},
		{"Given x, and when y, and only then z", true},
		// Whole words only.
		{"Given x, whenever y, then z", false},
		{"Given x, when y, thence z", false},
		{"Given x, somewhen y, then z", false},
		{"Given x, when y, athen z", false},
		{"Givenx, when y, then z", false},
		// It must start with "given".
		{"The first AC.", false},
		{"x given y, when z, then w", false},
		{"", false},
		{"   ", false},
	} {
		if got := GivenWhenThen(tc.description); got != tc.want {
			t.Errorf("GivenWhenThen(%q) = %v, want %v", tc.description, got, tc.want)
		}
	}
}

// lintIndex builds an index of the LINT feature: LINT-1 is active, with the
// given interface paths and ACs, and LINT-2 is a draft with the same.
func lintIndex(paths []string, acs ...parser.AcceptanceCriterion) *index.Index {
	rf := &parser.ReqFile{
		Feature:    "LINT",
		SourceFile: "/reqs/lint.req",
		Requirements: []parser.Requirement{
			{
				ID: "LINT-1", Title: "Active", Status: "active", Line: 3,
				Interface:          paths,
				AcceptanceCriteria: acs,
				Invariants: []parser.Invariant{
					{ID: "LINT-1-INV-1", Title: "Holds", Description: "It always holds.", Line: 20},
				},
			},
			{
				ID: "LINT-2", Title: "Draft", Status: "draft", Line: 30,
				Interface: paths,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "LINT-2-1", Title: "Draft AC", Description: "Not written as such.", Line: 33},
				},
			},
		},
	}
	return index.Build([]*parser.ReqFile{rf}, nil)
}

// SPEC-1-3
func TestMissingInterfaces(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg", "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "stub.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatal(err)
	}

	idx := lintIndex(
		[]string{"pkg/stub.go/x", "pkg/stub.go", "./pkg/missing.go", "pkg/sub", "pkg", "gone/dir"},
		parser.AcceptanceCriterion{ID: "LINT-1-1", Title: "One", Description: "Given x, when y, then z", Line: 10},
	)

	got := MissingInterfaces(idx, root)
	want := []spool.MissingInterface{
		{ID: "LINT-1", Path: "pkg/stub.go/x"},
		{ID: "LINT-1", Path: "./pkg/missing.go"},
		{ID: "LINT-1", Path: "gone/dir"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingInterfaces = %+v\nwant %+v", got, want)
	}
}

// SPEC-1-3
func TestMissingInterfaces_NoneMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "stub.go"), []byte("package x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][]string{nil, {"stub.go"}} {
		idx := lintIndex(paths)
		if got := MissingInterfaces(idx, root); len(got) != 0 {
			t.Errorf("interface %q: MissingInterfaces = %+v, want none", paths, got)
		}
	}
}

// SPEC-3-2
func TestNotGivenWhenThen(t *testing.T) {
	idx := lintIndex(nil,
		parser.AcceptanceCriterion{ID: "LINT-1-1", Title: "Formed", Description: "Given x,\nwhen y,\nthen z.\n", Line: 10},
		parser.AcceptanceCriterion{ID: "LINT-1-3", Title: "No When", Description: "Given x, then y", Line: 13},
		parser.AcceptanceCriterion{ID: "LINT-1-2", Title: "Unformed", Description: "Spool reports it.", Line: 16},
	)

	got := NotGivenWhenThen(idx)
	want := []spool.FormFinding{
		{ID: "LINT-1-3", Feature: "LINT", Title: "No When"},
		{ID: "LINT-1-2", Feature: "LINT", Title: "Unformed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NotGivenWhenThen = %+v\nwant %+v", got, want)
	}
}

// orderIndex builds an index of two features, given to it ZED first: ZED-1
// and, out of ID order, ALPHA-2 then ALPHA-1, all active, each with the
// given interface paths and ACs. The report orders them ALPHA-2, ALPHA-1,
// ZED-1 (SPEC-2-2).
func orderIndex(paths map[string][]string, acs map[string][]parser.AcceptanceCriterion) *index.Index {
	r := func(id string, line int) parser.Requirement {
		return parser.Requirement{
			ID: id, Title: id, Status: "active", Line: line,
			Interface:          paths[id],
			AcceptanceCriteria: acs[id],
		}
	}
	return index.Build([]*parser.ReqFile{
		{Feature: "ZED", SourceFile: "/reqs/zed.req", Requirements: []parser.Requirement{r("ZED-1", 3)}},
		{Feature: "ALPHA", SourceFile: "/reqs/alpha.req", Requirements: []parser.Requirement{r("ALPHA-2", 3), r("ALPHA-1", 20)}},
	}, nil)
}

// SPEC-1-3
func TestMissingInterfaces_Order(t *testing.T) {
	idx := orderIndex(map[string][]string{
		"ZED-1":   {"a/zed.go"},
		"ALPHA-2": {"z/two.go", "b/two.go"},
		"ALPHA-1": {"y/one.go", "a/one.go"},
	}, nil)

	got := MissingInterfaces(idx, t.TempDir())
	want := []spool.MissingInterface{
		{ID: "ALPHA-2", Path: "z/two.go"},
		{ID: "ALPHA-2", Path: "b/two.go"},
		{ID: "ALPHA-1", Path: "y/one.go"},
		{ID: "ALPHA-1", Path: "a/one.go"},
		{ID: "ZED-1", Path: "a/zed.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingInterfaces = %+v\nwant %+v", got, want)
	}
}

// SPEC-3-2
func TestNotGivenWhenThen_Order(t *testing.T) {
	unformed := func(id string, line int) parser.AcceptanceCriterion {
		return parser.AcceptanceCriterion{ID: id, Title: "T " + id, Description: "Spool reports it.", Line: line}
	}
	idx := orderIndex(nil, map[string][]parser.AcceptanceCriterion{
		"ZED-1":   {unformed("ZED-1-1", 6)},
		"ALPHA-2": {unformed("ALPHA-2-2", 6), unformed("ALPHA-2-1", 9)},
		"ALPHA-1": {unformed("ALPHA-1-1", 23)},
	})

	got := NotGivenWhenThen(idx)
	want := []spool.FormFinding{
		{ID: "ALPHA-2-2", Feature: "ALPHA", Title: "T ALPHA-2-2"},
		{ID: "ALPHA-2-1", Feature: "ALPHA", Title: "T ALPHA-2-1"},
		{ID: "ALPHA-1-1", Feature: "ALPHA", Title: "T ALPHA-1-1"},
		{ID: "ZED-1-1", Feature: "ZED", Title: "T ZED-1-1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NotGivenWhenThen = %+v\nwant %+v", got, want)
	}
}
