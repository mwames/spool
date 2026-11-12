package parser

import (
	"path/filepath"
	"strings"
	"testing"
)

// parseStatusReq writes a .req file of the STAT feature holding the given
// requirements, indented as list items under requirements, and parses it.
func parseStatusReq(t *testing.T, requirements string) (*ReqFile, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stat.req")
	writeReqFile(t, path, "feature: STAT\n\nrequirements:\n"+requirements)
	return ParseFile(path)
}

// itemStatuses returns the status of every requirement, AC and invariant in
// rf by ID.
func itemStatuses(rf *ReqFile) map[string]string {
	out := map[string]string{}
	for _, r := range rf.Requirements {
		out[r.ID] = r.Status
		for _, a := range r.AcceptanceCriteria {
			out[a.ID] = a.Status
		}
		for _, v := range r.Invariants {
			out[v.ID] = v.Status
		}
	}
	return out
}

// statusReqs has a requirement for each written status, each with ACs and
// invariants that have a status of their own and ones that have none.
const statusReqs = `
  - id: STAT-1
    title: Active
    description: Written active.
    status: active
    acceptance_criteria:
      - id: STAT-1-1
        title: Own draft
        description: Given a, when b, then c.
        status: draft
      - id: STAT-1-2
        title: Own active
        description: Given a, when b, then c.
        status: active
      - id: STAT-1-3
        title: Own deprecated
        description: Given a, when b, then c.
        status: deprecated
      - id: STAT-1-4
        title: None
        description: Given a, when b, then c.
    invariants:
      - id: STAT-1-INV-1
        title: Own draft
        description: It holds.
        status: draft
      - id: STAT-1-INV-2
        title: None
        description: It holds.

  - id: STAT-2
    title: Deprecated
    description: Written deprecated.
    status: deprecated
    acceptance_criteria:
      - id: STAT-2-1
        title: None
        description: Given a, when b, then c.
      - id: STAT-2-2
        title: Own active
        description: Given a, when b, then c.
        status: active
    invariants:
      - id: STAT-2-INV-1
        title: None
        description: It holds.

  - id: STAT-3
    title: Superseded
    description: Written superseded.
    status: superseded
    acceptance_criteria:
      - id: STAT-3-1
        title: None
        description: Given a, when b, then c.
      - id: STAT-3-2
        title: Own active
        description: Given a, when b, then c.
        status: active
    invariants:
      - id: STAT-3-INV-1
        title: Own deprecated
        description: It holds.
        status: deprecated

  - id: STAT-4
    title: Draft
    description: Written draft.
    status: draft
    acceptance_criteria:
      - id: STAT-4-1
        title: None
        description: Given a, when b, then c.
      - id: STAT-4-2
        title: Own active
        description: Given a, when b, then c.
        status: active
    invariants:
      - id: STAT-4-INV-1
        title: None
        description: It holds.
`

// SPEC-4-1
func TestParseFile_ItemStatus(t *testing.T) {
	rf, err := parseStatusReq(t, statusReqs)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	got := itemStatuses(rf)
	for id, want := range map[string]string{
		"STAT-1-1":     "draft",
		"STAT-1-2":     "active",
		"STAT-1-3":     "deprecated",
		"STAT-1-4":     "active", // the requirement's as written, not its folded draft
		"STAT-1-INV-1": "draft",
		"STAT-1-INV-2": "active",
		"STAT-2-1":     "deprecated",
		"STAT-2-2":     "active",
		"STAT-2-INV-1": "deprecated",
		"STAT-3-1":     "superseded",
		"STAT-3-2":     "active",
		"STAT-3-INV-1": "deprecated",
		"STAT-4-1":     "draft",
		"STAT-4-2":     "active",
		"STAT-4-INV-1": "draft",
	} {
		if got[id] != want {
			t.Errorf("%s status = %q, want %q", id, got[id], want)
		}
	}
}

// SPEC-4-4
func TestParseFile_FoldedRequirementStatus(t *testing.T) {
	rf, err := parseStatusReq(t, statusReqs)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	got := itemStatuses(rf)
	for id, want := range map[string]string{
		"STAT-1": "draft",      // active, draft and deprecated items
		"STAT-2": "active",     // deprecated and active items
		"STAT-3": "superseded", // written superseded, whatever its items
		"STAT-4": "draft",      // draft items and an active one
	} {
		if got[id] != want {
			t.Errorf("%s status = %q, want %q", id, got[id], want)
		}
	}
}

// SPEC-4-4
func TestParseFile_FoldedRequirementStatusCases(t *testing.T) {
	ac := func(id, status string) string {
		s := "      - id: " + id + "\n        title: AC\n        description: Given a, when b, then c.\n"
		if status != "" {
			s += "        status: " + status + "\n"
		}
		return s
	}
	inv := func(id, status string) string {
		s := "      - id: " + id + "\n        title: Inv\n        description: It holds.\n"
		if status != "" {
			s += "        status: " + status + "\n"
		}
		return s
	}
	for _, tc := range []struct {
		name    string
		written string
		acs     []string
		invs    []string
		want    string
	}{
		{"active with an active and a draft AC", "active", []string{ac("STAT-1-1", ""), ac("STAT-1-2", "draft")}, nil, "draft"},
		{"active with only deprecated ACs", "active", []string{ac("STAT-1-1", "deprecated"), ac("STAT-1-2", "deprecated")}, nil, "deprecated"},
		{"active with a deprecated and an inherited AC", "active", []string{ac("STAT-1-1", "deprecated"), ac("STAT-1-2", "")}, nil, "active"},
		{"deprecated with an active AC", "deprecated", []string{ac("STAT-1-1", "active")}, nil, "active"},
		{"deprecated with an inherited AC and an active invariant", "deprecated", []string{ac("STAT-1-1", "")}, []string{inv("STAT-1-INV-1", "active")}, "active"},
		{"draft with active and deprecated ACs", "draft", []string{ac("STAT-1-1", "active"), ac("STAT-1-2", "deprecated")}, nil, "active"},
		{"draft with an inherited AC", "draft", []string{ac("STAT-1-1", "")}, nil, "draft"},
		{"active with an active AC and a draft invariant", "active", []string{ac("STAT-1-1", "active")}, []string{inv("STAT-1-INV-1", "draft")}, "draft"},
		{"active with only an invariant, deprecated", "active", nil, []string{inv("STAT-1-INV-1", "deprecated")}, "deprecated"},
		{"superseded with draft and active ACs", "superseded", []string{ac("STAT-1-1", "draft"), ac("STAT-1-2", "active")}, nil, "superseded"},
		{"active with no items", "active", nil, nil, "active"},
		{"draft with no items", "draft", nil, nil, "draft"},
		{"deprecated with no items", "deprecated", nil, nil, "deprecated"},
		{"superseded with no items", "superseded", nil, nil, "superseded"},
		{"none with an active AC and a deprecated invariant", "", []string{ac("STAT-1-1", "active")}, []string{inv("STAT-1-INV-1", "deprecated")}, "active"},
		{"none with draft and deprecated ACs", "", []string{ac("STAT-1-1", "deprecated"), ac("STAT-1-2", "draft")}, nil, "draft"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := "  - id: STAT-1\n    title: Req\n    description: A requirement.\n"
			if tc.written != "" {
				y += "    status: " + tc.written + "\n"
			}
			if len(tc.acs) > 0 {
				y += "    acceptance_criteria:\n" + strings.Join(tc.acs, "")
			}
			if len(tc.invs) > 0 {
				y += "    invariants:\n" + strings.Join(tc.invs, "")
			}
			rf, err := parseStatusReq(t, y)
			if err != nil {
				t.Fatalf("ParseFile: %v\n%s", err, y)
			}
			if len(rf.Requirements) != 1 {
				t.Fatalf("len(Requirements) = %d, want 1", len(rf.Requirements))
			}
			if got := rf.Requirements[0].Status; got != tc.want {
				t.Errorf("status = %q, want %q\n%s", got, tc.want, y)
			}
		})
	}
}

// SPEC-4-4
func TestParseFile_FoldingLeavesItemStatus(t *testing.T) {
	rf, err := parseStatusReq(t, `
  - id: STAT-1
    title: Req
    description: Written active, folded draft.
    status: active
    acceptance_criteria:
      - id: STAT-1-1
        title: Inherited
        description: Given a, when b, then c.
      - id: STAT-1-2
        title: Draft
        description: Given a, when b, then c.
        status: draft
    invariants:
      - id: STAT-1-INV-1
        title: Inherited
        description: It holds.
`)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	got := itemStatuses(rf)
	for id, want := range map[string]string{"STAT-1": "draft", "STAT-1-1": "active", "STAT-1-2": "draft", "STAT-1-INV-1": "active"} {
		if got[id] != want {
			t.Errorf("%s status = %q, want %q", id, got[id], want)
		}
	}
}

// SPEC-4-2
func TestParseFile_UnknownStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		yaml    string
		id      string
		written string
	}{
		{"requirement", `
  - id: STAT-1
    title: Req
    description: A requirement.
    status: approved
    acceptance_criteria:
      - id: STAT-1-1
        title: AC
        description: Given a, when b, then c.
`, "STAT-1", "approved"},
		{"second requirement", `
  - id: STAT-1
    title: Req
    description: A requirement.
    status: active
  - id: STAT-2
    title: Req
    description: A requirement.
    status: proposed
`, "STAT-2", "proposed"},
		{"requirement with items of their own", `
  - id: STAT-1
    title: Req
    description: A requirement.
    status: retired
    acceptance_criteria:
      - id: STAT-1-1
        title: AC
        description: Given a, when b, then c.
        status: active
`, "STAT-1", "retired"},
		{"AC", `
  - id: STAT-1
    title: Req
    description: A requirement.
    status: active
    acceptance_criteria:
      - id: STAT-1-1
        title: AC
        description: Given a, when b, then c.
      - id: STAT-1-2
        title: AC
        description: Given a, when b, then c.
        status: approved
`, "STAT-1-2", "approved"},
		{"AC superseded", `
  - id: STAT-1
    title: Req
    description: A requirement.
    status: active
    acceptance_criteria:
      - id: STAT-1-1
        title: AC
        description: Given a, when b, then c.
        status: superseded
`, "STAT-1-1", "superseded"},
		{"invariant", `
  - id: STAT-1
    title: Req
    description: A requirement.
    status: active
    acceptance_criteria:
      - id: STAT-1-1
        title: AC
        description: Given a, when b, then c.
    invariants:
      - id: STAT-1-INV-1
        title: Inv
        description: It holds.
        status: pending
`, "STAT-1-INV-1", "pending"},
		{"invariant superseded", `
  - id: STAT-1
    title: Req
    description: A requirement.
    status: active
    invariants:
      - id: STAT-1-INV-1
        title: Inv
        description: It holds.
        status: superseded
`, "STAT-1-INV-1", "superseded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseStatusReq(t, tc.yaml)
			if err == nil {
				t.Fatalf("ParseFile succeeded, want an error naming %s and %q", tc.id, tc.written)
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.id) {
				t.Errorf("error %q does not name %s", msg, tc.id)
			}
			if !strings.Contains(msg, tc.written) {
				t.Errorf("error %q does not give the status as written, %q", msg, tc.written)
			}
		})
	}
}

// SPEC-4-2
func TestParseDir_UnknownStatusFailsFile(t *testing.T) {
	dir := t.TempDir()
	writeReqFile(t, filepath.Join(dir, "good.req"), "feature: GOOD\nrequirements:\n  - id: GOOD-1\n    title: Good\n    description: Fine.\n    status: active\n")
	writeReqFile(t, filepath.Join(dir, "bad.req"), "feature: BAD\nrequirements:\n  - id: BAD-1\n    title: Bad\n    description: Typo.\n    status: actve\n")

	result, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Feature != "GOOD" {
		t.Errorf("Files = %+v, want only good.req", result.Files)
	}
	if len(result.Errors) != 1 || !strings.HasSuffix(result.Errors[0].Path, "bad.req") {
		t.Fatalf("Errors = %v, want one for bad.req", result.Errors)
	}
	if msg := result.Errors[0].Error(); !strings.Contains(msg, "BAD-1") || !strings.Contains(msg, "actve") {
		t.Errorf("error %q, want it to name BAD-1 and \"actve\"", msg)
	}
}

// SPEC-4-2
func TestParseFile_KnownStatusesParse(t *testing.T) {
	rf, err := parseStatusReq(t, statusReqs)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(rf.Requirements) != 4 {
		t.Errorf("len(Requirements) = %d, want 4", len(rf.Requirements))
	}
}

// SPEC-4-3
func TestParseFile_MissingRequirementStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
	}{
		{"no items", `
  - id: STAT-2
    title: Req
    description: No status and nothing under it.
`},
		{"empty item lists", `
  - id: STAT-2
    title: Req
    description: No status and nothing under it.
    acceptance_criteria: []
    invariants: []
`},
		{"an AC with no status", `
  - id: STAT-2
    title: Req
    description: No status.
    acceptance_criteria:
      - id: STAT-2-1
        title: AC
        description: Given a, when b, then c.
        status: active
      - id: STAT-2-2
        title: AC
        description: Given a, when b, then c.
`},
		{"an invariant with no status", `
  - id: STAT-2
    title: Req
    description: No status.
    acceptance_criteria:
      - id: STAT-2-1
        title: AC
        description: Given a, when b, then c.
        status: active
    invariants:
      - id: STAT-2-INV-1
        title: Inv
        description: It holds.
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := `
  - id: STAT-1
    title: Req
    description: A requirement with a status.
    status: active
`
			_, err := parseStatusReq(t, first+tc.yaml)
			if err == nil {
				t.Fatal("ParseFile succeeded, want an error naming STAT-2 and the status field")
			}
			msg := err.Error()
			if !strings.Contains(msg, "STAT-2") {
				t.Errorf("error %q does not name the requirement STAT-2", msg)
			}
			if !strings.Contains(msg, "status") {
				t.Errorf("error %q does not name the status field", msg)
			}
		})
	}
}

// SPEC-4-3
func TestParseFile_NoRequirementStatusWithItemStatuses(t *testing.T) {
	rf, err := parseStatusReq(t, `
  - id: STAT-1
    title: Req
    description: No status, every item has one.
    acceptance_criteria:
      - id: STAT-1-1
        title: AC
        description: Given a, when b, then c.
        status: active
      - id: STAT-1-2
        title: AC
        description: Given a, when b, then c.
        status: deprecated
    invariants:
      - id: STAT-1-INV-1
        title: Inv
        description: It holds.
        status: draft
`)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(rf.Requirements) != 1 || len(rf.Requirements[0].AcceptanceCriteria) != 2 || len(rf.Requirements[0].Invariants) != 1 {
		t.Errorf("Requirements = %+v, want STAT-1 with two ACs and an invariant", rf.Requirements)
	}
}

// SPEC-4-INV-1
func TestParseFile_RequirementStatusAlone(t *testing.T) {
	rf, err := parseStatusReq(t, `
  - id: STAT-1
    title: Active
    description: No item has a status.
    status: active
    acceptance_criteria:
      - id: STAT-1-1
        title: AC
        description: Given a, when b, then c.
    invariants:
      - id: STAT-1-INV-1
        title: Inv
        description: It holds.
  - id: STAT-2
    title: Draft
    description: No item has a status.
    status: draft
    acceptance_criteria:
      - id: STAT-2-1
        title: AC
        description: Given a, when b, then c.
  - id: STAT-3
    title: Deprecated
    description: No item has a status.
    status: deprecated
    invariants:
      - id: STAT-3-INV-1
        title: Inv
        description: It holds.
  - id: STAT-4
    title: Superseded
    description: No item has a status.
    status: superseded
    acceptance_criteria:
      - id: STAT-4-1
        title: AC
        description: Given a, when b, then c.
  - id: STAT-5
    title: Empty
    description: No items.
    status: draft
`)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	written := map[string]string{"STAT-1": "active", "STAT-2": "draft", "STAT-3": "deprecated", "STAT-4": "superseded", "STAT-5": "draft"}
	for _, r := range rf.Requirements {
		if r.Status != written[r.ID] {
			t.Errorf("%s status = %q, want %q as written", r.ID, r.Status, written[r.ID])
		}
		for _, a := range r.AcceptanceCriteria {
			if a.Status != written[r.ID] {
				t.Errorf("%s status = %q, want its requirement's %q", a.ID, a.Status, written[r.ID])
			}
		}
		for _, v := range r.Invariants {
			if v.Status != written[r.ID] {
				t.Errorf("%s status = %q, want its requirement's %q", v.ID, v.Status, written[r.ID])
			}
		}
	}
}
