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

// lintIndex builds an index of the LINT feature as parsed, each requirement
// with its folded status (SPEC-4-4) and each item with its own (SPEC-4-1):
// LINT-1 is active, with the given interface paths and ACs, and LINT-2 is a
// draft with the same paths and an AC not written as Given/When/Then.
// LINT-3, deprecated, and LINT-4, superseded with an active AC, have the same
// paths and such an AC too.
func lintIndex(paths []string, acs ...parser.AcceptanceCriterion) *index.Index {
	for i := range acs {
		if acs[i].Status == "" {
			acs[i].Status = "active"
		}
	}
	rf := &parser.ReqFile{
		Feature:    "LINT",
		SourceFile: "/reqs/lint.req",
		Requirements: []parser.Requirement{
			{
				ID: "LINT-1", Title: "Active", Status: "active", Line: 3,
				Interface:          paths,
				AcceptanceCriteria: acs,
				Invariants: []parser.Invariant{
					{ID: "LINT-1-INV-1", Title: "Holds", Description: "It always holds.", Status: "active", Line: 20},
				},
			},
			{
				ID: "LINT-2", Title: "Draft", Status: "draft", Line: 30,
				Interface: paths,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "LINT-2-1", Title: "Draft AC", Description: "Not written as such.", Status: "draft", Line: 33},
				},
			},
			{
				ID: "LINT-3", Title: "Deprecated", Status: "deprecated", Line: 40,
				Interface: paths,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "LINT-3-1", Title: "Deprecated AC", Description: "Not written as such.", Status: "deprecated", Line: 43},
				},
			},
			{
				ID: "LINT-4", Title: "Superseded", Status: "superseded", Line: 50,
				Interface: paths,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "LINT-4-1", Title: "Superseded AC", Description: "Not written as such.", Status: "active", Line: 53},
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
		{ID: "LINT-2", Path: "pkg/stub.go/x"},
		{ID: "LINT-2", Path: "./pkg/missing.go"},
		{ID: "LINT-2", Path: "gone/dir"},
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
		{ID: "LINT-2-1", Feature: "LINT", Title: "Draft AC"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NotGivenWhenThen = %+v\nwant %+v", got, want)
	}
}

// orderIndex builds an index of two features, given to it ZED first: ZED-1
// and, out of ID order, ALPHA-2 then ALPHA-1, all active, each with the
// given interface paths and ACs. Findings are ordered as the report's
// features, then as the requirements are in the .req file: ALPHA-2, ALPHA-1,
// then ZED-1.
func orderIndex(paths map[string][]string, acs map[string][]parser.AcceptanceCriterion) *index.Index {
	r := func(id string, line int) parser.Requirement {
		for i := range acs[id] {
			acs[id][i].Status = "active"
		}
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

// SPEC-3-2
func TestNotGivenWhenThen_ItemStatus(t *testing.T) {
	idx := lintIndex(nil,
		parser.AcceptanceCriterion{ID: "LINT-1-1", Title: "Active", Description: "Spool reports it.", Status: "active", Line: 10},
		parser.AcceptanceCriterion{ID: "LINT-1-2", Title: "Draft", Description: "Spool reports it.", Status: "draft", Line: 13},
		parser.AcceptanceCriterion{ID: "LINT-1-3", Title: "Deprecated", Description: "Spool skips it.", Status: "deprecated", Line: 16},
	)

	got := NotGivenWhenThen(idx)
	want := []spool.FormFinding{
		{ID: "LINT-1-1", Feature: "LINT", Title: "Active"},
		{ID: "LINT-1-2", Feature: "LINT", Title: "Draft"},
		{ID: "LINT-2-1", Feature: "LINT", Title: "Draft AC"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NotGivenWhenThen = %+v\nwant %+v", got, want)
	}
}

// SPEC-1-3
func TestMissingInterfaces_RequirementStatus(t *testing.T) {
	idx := lintIndex([]string{"gone.go"})

	got := MissingInterfaces(idx, t.TempDir())
	want := []spool.MissingInterface{
		{ID: "LINT-1", Path: "gone.go"},
		{ID: "LINT-2", Path: "gone.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingInterfaces = %+v\nwant %+v (none for the deprecated LINT-3 or the superseded LINT-4)", got, want)
	}
}

// draftOrderIndex builds an index of two features, given to it ZED first:
// ZED-1, active, and ALPHA-2, active, ALPHA-9, a draft with only draft ACs
// that the report doesn't count (SPEC-4-6), then ALPHA-1, active. Each has
// the given interface paths and ACs, every AC with its requirement's status.
func draftOrderIndex(paths map[string][]string, acs map[string][]parser.AcceptanceCriterion) *index.Index {
	r := func(id, status string, line int) parser.Requirement {
		for i := range acs[id] {
			acs[id][i].Status = status
		}
		return parser.Requirement{
			ID: id, Title: id, Status: status, Line: line,
			Interface:          paths[id],
			AcceptanceCriteria: acs[id],
		}
	}
	return index.Build([]*parser.ReqFile{
		{Feature: "ZED", SourceFile: "/reqs/zed.req", Requirements: []parser.Requirement{r("ZED-1", "active", 3)}},
		{Feature: "ALPHA", SourceFile: "/reqs/alpha.req", Requirements: []parser.Requirement{
			r("ALPHA-2", "active", 3), r("ALPHA-9", "draft", 20), r("ALPHA-1", "active", 40),
		}},
	}, nil)
}

// SPEC-1-3
func TestMissingInterfaces_DraftOrder(t *testing.T) {
	idx := draftOrderIndex(map[string][]string{
		"ZED-1":   {"a/zed.go"},
		"ALPHA-2": {"z/two.go"},
		"ALPHA-9": {"y/nine.go", "b/nine.go"},
		"ALPHA-1": {"a/one.go"},
	}, nil)

	got := MissingInterfaces(idx, t.TempDir())
	want := []spool.MissingInterface{
		{ID: "ALPHA-2", Path: "z/two.go"},
		{ID: "ALPHA-9", Path: "y/nine.go"},
		{ID: "ALPHA-9", Path: "b/nine.go"},
		{ID: "ALPHA-1", Path: "a/one.go"},
		{ID: "ZED-1", Path: "a/zed.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingInterfaces = %+v\nwant %+v", got, want)
	}
}

// SPEC-3-2
func TestNotGivenWhenThen_DraftOrder(t *testing.T) {
	unformed := func(id string, line int) parser.AcceptanceCriterion {
		return parser.AcceptanceCriterion{ID: id, Title: "T " + id, Description: "Spool reports it.", Line: line}
	}
	idx := draftOrderIndex(nil, map[string][]parser.AcceptanceCriterion{
		"ZED-1":   {unformed("ZED-1-1", 6)},
		"ALPHA-2": {unformed("ALPHA-2-1", 6)},
		"ALPHA-9": {unformed("ALPHA-9-2", 23), unformed("ALPHA-9-1", 26)},
		"ALPHA-1": {unformed("ALPHA-1-1", 43)},
	})

	got := NotGivenWhenThen(idx)
	want := []spool.FormFinding{
		{ID: "ALPHA-2-1", Feature: "ALPHA", Title: "T ALPHA-2-1"},
		{ID: "ALPHA-9-2", Feature: "ALPHA", Title: "T ALPHA-9-2"},
		{ID: "ALPHA-9-1", Feature: "ALPHA", Title: "T ALPHA-9-1"},
		{ID: "ALPHA-1-1", Feature: "ALPHA", Title: "T ALPHA-1-1"},
		{ID: "ZED-1-1", Feature: "ZED", Title: "T ZED-1-1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NotGivenWhenThen = %+v\nwant %+v", got, want)
	}
}
