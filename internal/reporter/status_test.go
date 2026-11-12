package reporter

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

func acStatus(id, status string) parser.AcceptanceCriterion {
	a := ac(id, id+" description")
	a.Status = status
	return a
}

func invStatus(id, status string) parser.Invariant {
	v := inv(id, id+" description")
	v.Status = status
	return v
}

// itemStatusIndex holds requirements as parsed, each requirement with its
// folded status (SPEC-4-4) and each item with its own (SPEC-4-1):
//
//   - AUTH-1, folded draft: ACs active tested, active untested, draft tested,
//     draft untested, deprecated tested and deprecated untested; invariants
//     active tested, draft untested and deprecated tested.
//   - AUTH-2, written deprecated and folded active: an active untested AC and
//     a deprecated tested one.
//   - AUTH-3, superseded: active ACs, one tested and one not, and an active
//     untested invariant.
//   - AUTH-4, draft: a draft untested AC.
//   - AUTH-5, deprecated: a deprecated tested AC.
//   - AUTH-6, AUTH-7 and AUTH-8: active, draft and deprecated, with no items.
//   - BILL-1, active: only an active tested invariant.
//   - BILL-2, draft: no items.
func itemStatusIndex() *index.Index {
	return buildIndex(
		[]*parser.ReqFile{
			reqFile("AUTH",
				withInvariants(req("AUTH-1", "draft",
					acStatus("AUTH-1-1", "active"),
					acStatus("AUTH-1-2", "active"),
					acStatus("AUTH-1-3", "draft"),
					acStatus("AUTH-1-4", "draft"),
					acStatus("AUTH-1-5", "deprecated"),
					acStatus("AUTH-1-6", "deprecated"),
				),
					invStatus("AUTH-1-INV-1", "active"),
					invStatus("AUTH-1-INV-2", "draft"),
					invStatus("AUTH-1-INV-3", "deprecated"),
				),
				req("AUTH-2", "active",
					acStatus("AUTH-2-1", "active"),
					acStatus("AUTH-2-2", "deprecated"),
				),
				withInvariants(req("AUTH-3", "superseded",
					acStatus("AUTH-3-1", "active"),
					acStatus("AUTH-3-2", "active"),
				),
					invStatus("AUTH-3-INV-1", "active"),
				),
				req("AUTH-4", "draft", acStatus("AUTH-4-1", "draft")),
				req("AUTH-5", "deprecated", acStatus("AUTH-5-1", "deprecated")),
				req("AUTH-6", "active"),
				req("AUTH-7", "draft"),
				req("AUTH-8", "deprecated"),
			),
			reqFile("BILL",
				withInvariants(req("BILL-1", "active"), invStatus("BILL-1-INV-1", "active")),
				req("BILL-2", "draft"),
			),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go",
				mapping("TestOne", 5, "AUTH-1-1", "AUTH-1-3", "AUTH-1-5", "AUTH-1-INV-1", "AUTH-1-INV-3"),
				mapping("TestTwo", 10, "AUTH-2-2", "AUTH-3-1", "AUTH-5-1"),
			),
			scanResult("bill_test.go", mapping("TestBill", 5, "BILL-1-INV-1")),
		},
	)
}

// SPEC-4-5
func TestGenerate_CountsOnlyActiveItems(t *testing.T) {
	r := Generate(itemStatusIndex())

	type counts struct{ ACs, LinkedACs, UntestedACs, Invariants, LinkedInvariants, UntestedInvariants int }
	got := counts{r.TotalACs, r.LinkedACs, r.UntestedACs, r.TotalInvariants, r.LinkedInvariants, r.UntestedInvariants}
	if want := (counts{3, 1, 2, 2, 2, 0}); got != want {
		t.Errorf("totals = %+v, want %+v", got, want)
	}

	want := map[string]counts{
		"AUTH": {3, 1, 2, 1, 1, 0},
		"BILL": {0, 0, 0, 1, 1, 0},
	}
	if len(r.Features) != len(want) {
		t.Fatalf("Features = %+v, want AUTH and BILL", r.Features)
	}
	for _, f := range r.Features {
		got := counts{f.ACs, f.LinkedACs, f.UntestedACs, f.Invariants, f.LinkedInvariants, f.UntestedInvariants}
		if got != want[f.Name] {
			t.Errorf("%s = %+v, want %+v", f.Name, got, want[f.Name])
		}
	}
}

// SPEC-4-5
func TestGenerate_ListsOnlyActiveUntestedItems(t *testing.T) {
	r := Generate(itemStatusIndex())

	var acs []string
	for _, u := range r.Untested {
		acs = append(acs, u.ID)
	}
	sort.Strings(acs)
	if want := []string{"AUTH-1-2", "AUTH-2-1"}; !reflect.DeepEqual(acs, want) {
		t.Errorf("Untested = %v, want %v", acs, want)
	}
	if len(r.UntestedInvariantList) != 0 {
		t.Errorf("UntestedInvariantList = %+v, want none", r.UntestedInvariantList)
	}
}

// SPEC-4-6
func TestGenerate_RequirementsCounted(t *testing.T) {
	r := Generate(itemStatusIndex())

	if r.TotalRequirements != 4 {
		t.Errorf("TotalRequirements = %d, want 4 (AUTH-1, AUTH-2, AUTH-6, BILL-1)", r.TotalRequirements)
	}
	want := map[string][2]int{ // requirements, excluded
		"AUTH": {3, 5},
		"BILL": {1, 1},
	}
	for _, f := range r.Features {
		if got := [2]int{f.Requirements, f.ExcludedRequirements}; got != want[f.Name] {
			t.Errorf("%s requirements/excluded = %v, want %v", f.Name, got, want[f.Name])
		}
	}
}

// SPEC-2-2
// SPEC-4-6
func TestGenerate_RequirementEntries(t *testing.T) {
	r := Generate(itemStatusIndex())

	type entry struct{ ID, Feature, Status string }
	var got []entry
	for _, q := range r.Requirements {
		got = append(got, entry{q.ID, q.Feature, q.Status})
	}
	want := []entry{
		{"AUTH-1", "AUTH", "draft"},
		{"AUTH-2", "AUTH", "active"},
		{"AUTH-6", "AUTH", "active"},
		{"BILL-1", "BILL", "active"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Requirements = %+v\nwant %+v", got, want)
	}
}

// SPEC-4-5
func TestExitCode_OnlyActiveItems(t *testing.T) {
	sev := config.SeverityConfig{
		UntestedAC:         config.SeverityError,
		UntestedInvariant:  config.SeverityError,
		OrphanedAnnotation: config.SeverityOff,
	}
	for _, tc := range []struct {
		name string
		reqs []parser.Requirement
		want int
	}{
		{"untested draft and deprecated items", []parser.Requirement{
			withInvariants(req("AUTH-1", "draft",
				acStatus("AUTH-1-1", "active"),
				acStatus("AUTH-1-2", "draft"),
			), invStatus("AUTH-1-INV-1", "deprecated"), invStatus("AUTH-1-INV-2", "draft")),
			withInvariants(req("AUTH-2", "active",
				acStatus("AUTH-2-1", "active"),
				acStatus("AUTH-2-2", "deprecated"),
			), invStatus("AUTH-2-INV-1", "deprecated")),
		}, 0},
		{"untested items of a superseded requirement", []parser.Requirement{
			req("AUTH-1", "active", acStatus("AUTH-1-1", "active")),
			withInvariants(req("AUTH-2", "superseded", acStatus("AUTH-2-1", "active"), acStatus("AUTH-2-2", "active")),
				invStatus("AUTH-2-INV-1", "active")),
		}, 0},
		{"untested active AC of a draft requirement", []parser.Requirement{
			req("AUTH-1", "draft",
				acStatus("AUTH-1-1", "active"),
				acStatus("AUTH-1-2", "draft"),
				acStatus("AUTH-1-9", "active"),
			),
		}, 1},
		{"untested active invariant of a draft requirement", []parser.Requirement{
			withInvariants(req("AUTH-1", "draft", acStatus("AUTH-1-1", "active")),
				invStatus("AUTH-1-INV-9", "active"), invStatus("AUTH-1-INV-2", "draft")),
		}, 1},
		{"untested active AC of a requirement written deprecated", []parser.Requirement{
			req("AUTH-1", "active", acStatus("AUTH-1-1", "active")),
			req("AUTH-2", "active", acStatus("AUTH-2-9", "active"), acStatus("AUTH-2-2", "deprecated")),
		}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx := buildIndex(
				[]*parser.ReqFile{reqFile("AUTH", tc.reqs...)},
				[]scanner.FileResult{scanResult("auth_test.go",
					mapping("TestOne", 5, "AUTH-1-1", "AUTH-2-1", "AUTH-2-2")),
				},
			)
			if got := ExitCode(Generate(idx), sev); got != tc.want {
				t.Errorf("ExitCode = %d, want %d", got, tc.want)
			}
		})
	}
}

// SPEC-2-2
func TestJSONFormatter_RequirementStatus(t *testing.T) {
	r := &spool.Report{
		Requirements: []spool.RequirementEntry{
			{ID: "AUTH-1", Feature: "AUTH", Title: "Login", Status: "draft"},
			{ID: "AUTH-2", Feature: "AUTH", Title: "Logout", Status: "active", Interface: []string{"a.go"}, OutOfScope: []string{"Nothing"}},
		},
	}

	data, err := JSON{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		Requirements []map[string]any `json:"requirements"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Requirements) != 2 {
		t.Fatalf("requirements = %v, want 2 entries", parsed.Requirements)
	}
	wantKeys := []string{"feature", "id", "interface", "outOfScope", "status", "title"}
	for i, want := range []string{"draft", "active"} {
		e := parsed.Requirements[i]
		var keys []string
		for k := range e {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if !reflect.DeepEqual(keys, wantKeys) {
			t.Errorf("requirements[%d] keys = %v, want %v", i, keys, wantKeys)
		}
		if e["status"] != want {
			t.Errorf("requirements[%d].status = %v, want %q", i, e["status"], want)
		}
	}
}
