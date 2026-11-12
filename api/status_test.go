package api_test

import (
	"testing"

	"github.com/mwames/Spool/api"
)

// foldedStatusFixture has requirements whose folded status (SPEC-4-4)
// differs from the one written: AUTH-1 is written active with a draft AC,
// AUTH-2 written deprecated with an active AC, AUTH-3 superseded with draft
// and active ACs, and AUTH-4 written active with only deprecated ACs.
// BILL-1 is active with no AC of its own status.
func foldedStatusFixture(t *testing.T) string {
	t.Helper()
	return fixture{
		Features: []featureDef{
			{Name: "AUTH", Reqs: []reqDef{
				{ID: "AUTH-1", Status: "active", ACs: []acDef{
					{ID: "AUTH-1-1"},
					{ID: "AUTH-1-2", Status: "draft"},
				}},
				{ID: "AUTH-2", Status: "deprecated", ACs: []acDef{
					{ID: "AUTH-2-1", Status: "active"},
					{ID: "AUTH-2-2"},
				}},
				{ID: "AUTH-3", Status: "superseded", ACs: []acDef{
					{ID: "AUTH-3-1", Status: "draft"},
					{ID: "AUTH-3-2", Status: "active"},
				}},
				{ID: "AUTH-4", Status: "active", ACs: []acDef{
					{ID: "AUTH-4-1", Status: "deprecated"},
				}},
			}},
			{Name: "BILL", Reqs: []reqDef{
				{ID: "BILL-1", Status: "active", ACs: []acDef{
					{ID: "BILL-1-1"},
				}},
			}},
		},
	}.build(t)
}

var foldedStatuses = map[string]string{
	"AUTH-1": "draft",
	"AUTH-2": "active",
	"AUTH-3": "superseded",
	"AUTH-4": "deprecated",
	"BILL-1": "active",
}

// SPEC-4-4
func TestProject_RequirementViewsFoldedStatus(t *testing.T) {
	proj := mustLoad(t, foldedStatusFixture(t))
	if w := proj.Warnings(); len(w) != 0 {
		t.Fatalf("fixture: warnings = %+v, want none", w)
	}

	for id, want := range foldedStatuses {
		v := proj.Requirement(id)
		if v == nil {
			t.Errorf("Requirement(%s) is nil", id)
			continue
		}
		if v.Status != want {
			t.Errorf("Requirement(%s).Status = %q, want %q", id, v.Status, want)
		}
	}

	list := proj.Requirements(api.RequirementFilter{})
	if len(list) != len(foldedStatuses) {
		t.Fatalf("Requirements = %+v, want %d", list, len(foldedStatuses))
	}
	for _, v := range list {
		if v.Status != foldedStatuses[v.ID] {
			t.Errorf("Requirements: %s Status = %q, want %q", v.ID, v.Status, foldedStatuses[v.ID])
		}
	}
}

// SPEC-4-4
func TestProject_RequirementsStatusFilterFolded(t *testing.T) {
	proj := mustLoad(t, foldedStatusFixture(t))

	for _, tc := range []struct {
		filter api.RequirementFilter
		want   []string
	}{
		{api.RequirementFilter{Status: "draft"}, []string{"AUTH-1"}},
		{api.RequirementFilter{Status: "active"}, []string{"AUTH-2", "BILL-1"}},
		{api.RequirementFilter{Status: "deprecated"}, []string{"AUTH-4"}},
		{api.RequirementFilter{Status: "superseded"}, []string{"AUTH-3"}},
		{api.RequirementFilter{Feature: "AUTH", Status: "active"}, []string{"AUTH-2"}},
	} {
		got := proj.Requirements(tc.filter)
		var ids []string
		for _, v := range got {
			ids = append(ids, v.ID)
		}
		if len(ids) != len(tc.want) {
			t.Errorf("Requirements(%+v) = %v, want %v", tc.filter, ids, tc.want)
			continue
		}
		for i := range ids {
			if ids[i] != tc.want[i] {
				t.Errorf("Requirements(%+v) = %v, want %v", tc.filter, ids, tc.want)
				break
			}
		}
	}
}
