package index

import (
	"sort"
	"testing"

	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

func acStatus(id, status string) parser.AcceptanceCriterion {
	a := ac(id)
	a.Status = status
	return a
}

func invStatus(id, status string) parser.Invariant {
	v := inv(id)
	v.Status = status
	return v
}

// itemStatusIndex holds requirements as parsed (SPEC-4-1, SPEC-4-4), each
// item with its own status, none of them tested except where noted:
//
//   - AUTH-1, written active and folded draft: an active, a draft and a
//     deprecated AC and invariant.
//   - AUTH-2, written deprecated and folded active: an active AC and
//     invariant, and an inherited deprecated AC.
//   - AUTH-3, superseded: an active AC and invariant.
//   - AUTH-4, written draft and folded active: an active AC that is tested,
//     and a draft one that is too.
func itemStatusIndex() *Index {
	return Build(
		[]*parser.ReqFile{
			reqFile("AUTH",
				withInvariants(req("AUTH-1", "draft",
					acStatus("AUTH-1-1", "active"),
					acStatus("AUTH-1-2", "draft"),
					acStatus("AUTH-1-3", "deprecated"),
				),
					invStatus("AUTH-1-INV-1", "active"),
					invStatus("AUTH-1-INV-2", "draft"),
					invStatus("AUTH-1-INV-3", "deprecated"),
				),
				withInvariants(req("AUTH-2", "active",
					acStatus("AUTH-2-1", "active"),
					acStatus("AUTH-2-2", "deprecated"),
				),
					invStatus("AUTH-2-INV-1", "active"),
				),
				withInvariants(req("AUTH-3", "superseded",
					acStatus("AUTH-3-1", "active"),
				),
					invStatus("AUTH-3-INV-1", "active"),
				),
				req("AUTH-4", "active",
					acStatus("AUTH-4-1", "active"),
					acStatus("AUTH-4-2", "draft"),
				),
			),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go",
				mapping("TestActive", 5, "AUTH-4-1"),
				mapping("TestDraft", 10, "AUTH-4-2"),
			),
		},
	)
}

func untestedACIDs(idx *Index) []string {
	var ids []string
	for _, a := range idx.UntestedACs() {
		ids = append(ids, a.ID)
	}
	sort.Strings(ids)
	return ids
}

func untestedInvariantIDs(idx *Index) []string {
	var ids []string
	for _, v := range idx.UntestedInvariants() {
		ids = append(ids, v.ID)
	}
	sort.Strings(ids)
	return ids
}

func has(ids []string, id string) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

// INDEX-4-1
// SPEC-4-5
func TestUntestedACs_ActiveACOfNonActiveRequirement(t *testing.T) {
	got := untestedACIDs(itemStatusIndex())
	for _, id := range []string{"AUTH-1-1", "AUTH-2-1"} {
		if !has(got, id) {
			t.Errorf("UntestedACs = %v, want it to hold %s, an active AC with no tests", got, id)
		}
	}
}

// INDEX-4-2
// SPEC-4-5
func TestUntestedACs_ActiveTestedACOfDraftRequirement(t *testing.T) {
	if got := untestedACIDs(itemStatusIndex()); has(got, "AUTH-4-1") {
		t.Errorf("UntestedACs = %v, want no AUTH-4-1, which is tested", got)
	}
}

// INDEX-4-3
// SPEC-4-5
func TestUntestedACs_NonActiveItemOfActiveRequirement(t *testing.T) {
	got := untestedACIDs(itemStatusIndex())
	for _, id := range []string{"AUTH-1-2", "AUTH-1-3", "AUTH-2-2", "AUTH-3-1", "AUTH-4-2"} {
		if has(got, id) {
			t.Errorf("UntestedACs = %v, want no %s, which isn't active", got, id)
		}
	}
	if want := []string{"AUTH-1-1", "AUTH-2-1"}; len(got) != len(want) {
		t.Errorf("UntestedACs = %v, want %v", got, want)
	}
}

// INDEX-4-3
// SPEC-4-5
func TestUntestedACs_OnlyNonActiveUntested(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH",
				req("AUTH-1", "draft", acStatus("AUTH-1-1", "draft"), acStatus("AUTH-1-2", "active")),
				req("AUTH-2", "deprecated", acStatus("AUTH-2-1", "deprecated")),
			),
		},
		[]scanner.FileResult{scanResult("auth_test.go", mapping("TestTwo", 5, "AUTH-1-2"))},
	)
	if got := untestedACIDs(idx); len(got) != 0 {
		t.Errorf("UntestedACs = %v, want none", got)
	}
}

// INVARIANT-1-5
// SPEC-4-5
func TestUntestedInvariants_ItemStatus(t *testing.T) {
	got := untestedInvariantIDs(itemStatusIndex())
	want := []string{"AUTH-1-INV-1", "AUTH-2-INV-1"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("UntestedInvariants = %v, want %v", got, want)
	}
}
