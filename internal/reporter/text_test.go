package reporter

import (
	"strings"
	"testing"

	spool "github.com/mwames/Spool"
)

// REPORTER-3-1: Summary Table
func TestTextFormatter_SummaryTable(t *testing.T) {
	r := &spool.Report{
		TotalRequirements: 3,
		TotalACs:          5,
		LinkedACs:         3,
		UntestedACs:       2,
		OrphanedCount:     1,
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 2, ACs: 3, LinkedACs: 2, UntestedACs: 1},
			{Name: "CONFIG", Requirements: 1, ACs: 2, LinkedACs: 1, UntestedACs: 1},
		},
	}

	data, err := Text{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "AUTH") {
		t.Error("output should contain AUTH")
	}
	if !strings.Contains(out, "CONFIG") {
		t.Error("output should contain CONFIG")
	}
}

// REPORTER-3-2: Untested ACs Listed
func TestTextFormatter_UntestedListed(t *testing.T) {
	r := &spool.Report{
		Untested: []spool.UntestedAC{
			{ID: "AUTH-1-1", Description: "must login", Feature: "AUTH"},
		},
	}

	data, err := Text{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "AUTH-1-1") {
		t.Error("output should contain AUTH-1-1")
	}
	if !strings.Contains(out, "must login") {
		t.Error("output should contain description")
	}
}

// REPORTER-3-3: Orphaned Listed
func TestTextFormatter_OrphanedListed(t *testing.T) {
	r := &spool.Report{
		Orphaned: []spool.OrphanedRef{
			{ID: "AUTH-99-1", File: "auth_test.go", Line: 5},
		},
	}

	data, err := Text{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "AUTH-99-1") {
		t.Error("output should contain AUTH-99-1")
	}
	if !strings.Contains(out, "auth_test.go") {
		t.Error("output should contain file path")
	}
	if !strings.Contains(out, "5") {
		t.Error("output should contain line number")
	}
}

// REPORTER-3-4: No Issues
func TestTextFormatter_NoIssues(t *testing.T) {
	r := &spool.Report{
		TotalRequirements: 1,
		TotalACs:          1,
		LinkedACs:         1,
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 1, ACs: 1, LinkedACs: 1},
		},
	}

	data, err := Text{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := strings.ToLower(string(data))
	if !strings.Contains(out, "no issues") && !strings.Contains(out, "full traceability") {
		t.Error("output should indicate no issues found")
	}
}

// REPORTER-3-5: Excluded Reqs
func TestTextFormatter_ExcludedReqs(t *testing.T) {
	r := &spool.Report{
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 1, ACs: 1, ExcludedRequirements: 2},
		},
	}

	data, err := Text{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "2") {
		t.Error("output should show excluded count")
	}
}
