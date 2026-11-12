package cli

import (
	"strings"
	"testing"
)

// itemReq has requirements whose ACs and invariants have statuses of their
// own (SPEC-4-1). Every AC is written as Given/When/Then.
//
//   - ITEM-1, active: ACs inherited active tested and untested, draft tested
//     and untested, deprecated tested and untested; invariants active
//     untested, draft untested and deprecated tested.
//   - ITEM-2, written deprecated: an active tested AC and an inherited
//     deprecated untested one.
//   - ITEM-3, superseded: an active untested AC and invariant.
//   - ITEM-4, draft: an inherited draft untested AC.
//   - ITEM-5, active with no items.
//   - ITEM-6, written active with only a deprecated untested AC.
const itemReq = `feature: ITEM

requirements:

  - id: ITEM-1
    title: Mixed
    description: Items of every status.
    status: active
    acceptance_criteria:
      - id: ITEM-1-1
        title: Active Tested
        description: Given a, when b, then c.
      - id: ITEM-1-2
        title: Active Untested
        description: Given a, when b, then c.
      - id: ITEM-1-3
        title: Draft Tested
        description: Given a, when b, then c.
        status: draft
      - id: ITEM-1-4
        title: Draft Untested
        description: Given a, when b, then c.
        status: draft
      - id: ITEM-1-5
        title: Deprecated Tested
        description: Given a, when b, then c.
        status: deprecated
      - id: ITEM-1-6
        title: Deprecated Untested
        description: Given a, when b, then c.
        status: deprecated
    invariants:
      - id: ITEM-1-INV-1
        title: Active Untested
        description: It holds.
      - id: ITEM-1-INV-2
        title: Draft Untested
        description: It holds.
        status: draft
      - id: ITEM-1-INV-3
        title: Deprecated Tested
        description: It holds.
        status: deprecated

  - id: ITEM-2
    title: Revived
    description: Written deprecated.
    status: deprecated
    acceptance_criteria:
      - id: ITEM-2-1
        title: Active Tested
        description: Given a, when b, then c.
        status: active
      - id: ITEM-2-2
        title: Inherited Untested
        description: Given a, when b, then c.

  - id: ITEM-3
    title: Superseded
    description: Superseded.
    status: superseded
    acceptance_criteria:
      - id: ITEM-3-1
        title: Active Untested
        description: Given a, when b, then c.
        status: active
    invariants:
      - id: ITEM-3-INV-1
        title: Active Untested
        description: It holds.
        status: active

  - id: ITEM-4
    title: Draft
    description: A draft.
    status: draft
    acceptance_criteria:
      - id: ITEM-4-1
        title: Inherited Untested
        description: Given a, when b, then c.

  - id: ITEM-5
    title: Empty
    description: Active with no items.
    status: active

  - id: ITEM-6
    title: Retired
    description: Written active, with only a deprecated AC.
    status: active
    acceptance_criteria:
      - id: ITEM-6-1
        title: Deprecated Untested
        description: Given a, when b, then c.
        status: deprecated
`

// itemProject writes itemReq with tests for ITEM-1-1, ITEM-1-3, ITEM-1-5,
// ITEM-1-INV-3 and ITEM-2-1, and with the given further tests.
func itemProject(t *testing.T, more ...string) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".spool.yaml":      lintConfig("", ""),
		".spool/item.req":  itemReq,
		"pkg/item_test.go": annotatedTest(append([]string{"ITEM-1-1", "ITEM-1-3", "ITEM-1-5", "ITEM-1-INV-3", "ITEM-2-1"}, more...)...),
	})
	return root
}

type statusCounts struct {
	TotalRequirements  int `json:"totalRequirements"`
	TotalACs           int `json:"totalACs"`
	LinkedACs          int `json:"linkedACs"`
	UntestedACs        int `json:"untestedACs"`
	OrphanedCount      int `json:"orphanedCount"`
	TotalInvariants    int `json:"totalInvariants"`
	LinkedInvariants   int `json:"linkedInvariants"`
	UntestedInvariants int `json:"untestedInvariants"`
	Features           []struct {
		Name                 string `json:"name"`
		Requirements         int    `json:"requirements"`
		ExcludedRequirements int    `json:"excludedRequirements"`
		ACs                  int    `json:"acs"`
		LinkedACs            int    `json:"linkedACs"`
		UntestedACs          int    `json:"untestedACs"`
		Invariants           int    `json:"invariants"`
		LinkedInvariants     int    `json:"linkedInvariants"`
		UntestedInvariants   int    `json:"untestedInvariants"`
	} `json:"features"`
	Untested []struct {
		ID string `json:"id"`
	} `json:"untested"`
	UntestedInvariantList []struct {
		ID string `json:"id"`
	} `json:"untestedInvariantList"`
}

func untestedIDs(c statusCounts) (acs, invs []string) {
	for _, u := range c.Untested {
		acs = append(acs, u.ID)
	}
	for _, u := range c.UntestedInvariantList {
		invs = append(invs, u.ID)
	}
	return acs, invs
}

// SPEC-4-5
func TestRun_CoverageByItem(t *testing.T) {
	root := itemProject(t)
	code, stdout, stderr := run(t, root, "--format", "json")
	var c statusCounts
	decodeOne(t, stdout, &c)

	type totals struct{ ACs, LinkedACs, UntestedACs, Invariants, LinkedInvariants, UntestedInvariants int }
	if got, want := (totals{c.TotalACs, c.LinkedACs, c.UntestedACs, c.TotalInvariants, c.LinkedInvariants, c.UntestedInvariants}), (totals{3, 2, 1, 1, 0, 1}); got != want {
		t.Errorf("totals = %+v, want %+v\n%s", got, want, stdout)
	}
	if len(c.Features) != 1 {
		t.Fatalf("features = %+v, want ITEM only", c.Features)
	}
	f := c.Features[0]
	if got, want := (totals{f.ACs, f.LinkedACs, f.UntestedACs, f.Invariants, f.LinkedInvariants, f.UntestedInvariants}), (totals{3, 2, 1, 1, 0, 1}); got != want {
		t.Errorf("ITEM = %+v, want %+v", got, want)
	}
	acs, invs := untestedIDs(c)
	if strings.Join(acs, ",") != "ITEM-1-2" {
		t.Errorf("untested = %v, want [ITEM-1-2]", acs)
	}
	if strings.Join(invs, ",") != "ITEM-1-INV-1" {
		t.Errorf("untestedInvariantList = %v, want [ITEM-1-INV-1]", invs)
	}
	if c.OrphanedCount != 0 {
		t.Errorf("orphanedCount = %d, want 0: tests of draft and deprecated items are not orphaned", c.OrphanedCount)
	}
	if code != 1 {
		t.Errorf("exit = %d, want 1 for the untested active items\nstderr:\n%s", code, stderr)
	}

	_, text, _ := run(t, root)
	for _, id := range []string{"ITEM-1-4", "ITEM-1-6", "ITEM-1-INV-2", "ITEM-2-2", "ITEM-3-1", "ITEM-3-INV-1", "ITEM-4-1", "ITEM-6-1"} {
		if strings.Contains(text, id) {
			t.Errorf("text report lists %s, which isn't active:\n%s", id, text)
		}
	}
}

// SPEC-4-5
func TestRun_OnlyActiveItemsAffectExit(t *testing.T) {
	code, stdout, stderr := run(t, itemProject(t, "ITEM-1-2", "ITEM-1-INV-1"))
	if code != 0 {
		t.Errorf("exit = %d, want 0 with every active item tested\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.HasSuffix(strings.TrimRight(stdout, "\n"), noIssuesLine) {
		t.Errorf("report does not end with %q with every active item tested:\n%s", noIssuesLine, stdout)
	}
}

// SPEC-2-2
// SPEC-4-6
func TestRun_RequirementsCounted(t *testing.T) {
	_, stdout, _ := run(t, itemProject(t), "--format", "json")
	var c statusCounts
	decodeOne(t, stdout, &c)

	// ITEM-1, ITEM-2 and ITEM-5 are counted; ITEM-3, ITEM-4 and ITEM-6 are
	// excluded.
	if c.TotalRequirements != 3 {
		t.Errorf("totalRequirements = %d, want 3", c.TotalRequirements)
	}
	if len(c.Features) != 1 {
		t.Fatalf("features = %+v, want ITEM only", c.Features)
	}
	if f := c.Features[0]; f.Requirements != 3 || f.ExcludedRequirements != 3 {
		t.Errorf("ITEM requirements/excluded = %d/%d, want 3/3", f.Requirements, f.ExcludedRequirements)
	}

	var got []string
	for _, raw := range jsonList(t, stdout, "requirements", []string{"feature", "id", "interface", "outOfScope", "status", "title"}) {
		var r jsonRequirement
		decodeOne(t, string(raw), &r)
		got = append(got, r.ID+":"+r.Status)
	}
	// ITEM-1 is folded draft and ITEM-2 active (SPEC-4-4).
	if want := "ITEM-1:draft,ITEM-2:active,ITEM-5:active"; strings.Join(got, ",") != want {
		t.Errorf("requirements = %v, want %s", got, want)
	}
}

// aloneReq is a project in which no AC or invariant has a status of its own.
const aloneReq = `feature: ALONE

requirements:

  - id: ALONE-1
    title: Active
    description: Active.
    status: active
    acceptance_criteria:
      - id: ALONE-1-1
        title: Tested
        description: Given a, when b, then c.
      - id: ALONE-1-2
        title: Untested
        description: Given a, when b, then c.
    invariants:
      - id: ALONE-1-INV-1
        title: Untested
        description: It holds.

  - id: ALONE-2
    title: Draft
    description: Draft.
    status: draft
    acceptance_criteria:
      - id: ALONE-2-1
        title: Untested
        description: Given a, when b, then c.

  - id: ALONE-3
    title: Deprecated
    description: Deprecated.
    status: deprecated
    acceptance_criteria:
      - id: ALONE-3-1
        title: Tested
        description: Given a, when b, then c.
    invariants:
      - id: ALONE-3-INV-1
        title: Untested
        description: It holds.

  - id: ALONE-4
    title: Superseded
    description: Superseded.
    status: superseded
    acceptance_criteria:
      - id: ALONE-4-1
        title: Untested
        description: Given a, when b, then c.

  - id: ALONE-5
    title: Empty Active
    description: Active with no items.
    status: active

  - id: ALONE-6
    title: Empty Draft
    description: Draft with no items.
    status: draft
`

// SPEC-4-INV-1
func TestRun_RequirementStatusAlone(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".spool.yaml":       lintConfig("", ""),
		".spool/alone.req":  aloneReq,
		"pkg/alone_test.go": annotatedTest("ALONE-1-1", "ALONE-3-1"),
	})
	code, stdout, _ := run(t, root, "--format", "json")

	var c statusCounts
	decodeOne(t, stdout, &c)
	if c.TotalRequirements != 2 || c.TotalACs != 2 || c.LinkedACs != 1 || c.UntestedACs != 1 || c.TotalInvariants != 1 || c.UntestedInvariants != 1 {
		t.Errorf("counts = %+v, want 2 requirements, 2 ACs (1 linked, 1 untested), 1 untested invariant", c)
	}
	if len(c.Features) == 1 && c.Features[0].ExcludedRequirements != 4 {
		t.Errorf("excludedRequirements = %d, want 4", c.Features[0].ExcludedRequirements)
	}
	acs, invs := untestedIDs(c)
	if strings.Join(acs, ",") != "ALONE-1-2" || strings.Join(invs, ",") != "ALONE-1-INV-1" {
		t.Errorf("untested = %v and %v, want [ALONE-1-2] and [ALONE-1-INV-1]", acs, invs)
	}
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}

	var got []string
	for _, raw := range jsonList(t, stdout, "requirements", []string{"feature", "id", "interface", "outOfScope", "status", "title"}) {
		var r jsonRequirement
		decodeOne(t, string(raw), &r)
		got = append(got, r.ID+":"+r.Status)
	}
	if want := "ALONE-1:active,ALONE-5:active"; strings.Join(got, ",") != want {
		t.Errorf("requirements = %v, want %s", got, want)
	}
}
