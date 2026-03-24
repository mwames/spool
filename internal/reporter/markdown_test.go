package reporter

import (
	"strings"
	"testing"

	spool "github.com/mwames/Spool"
)

// REPORTER-5-1: Markdown Table
func TestMarkdownFormatter_Table(t *testing.T) {
	r := &spool.Report{
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 2, ACs: 3, LinkedACs: 2, UntestedACs: 1},
		},
	}

	data, err := Markdown{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "|") {
		t.Error("output should contain markdown table pipes")
	}
	if !strings.Contains(out, "AUTH") {
		t.Error("output should contain feature name")
	}
	if !strings.Contains(out, "---") {
		t.Error("output should contain table separator")
	}
}

// REPORTER-5-2: Untested Section
func TestMarkdownFormatter_UntestedSection(t *testing.T) {
	r := &spool.Report{
		Untested: []spool.UntestedAC{
			{ID: "AUTH-1-1", Description: "must login", Feature: "AUTH"},
		},
	}

	data, err := Markdown{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "AUTH-1-1") {
		t.Error("output should contain AC ID")
	}
	if !strings.Contains(out, "must login") {
		t.Error("output should contain description")
	}
}

// REPORTER-5-3: Orphaned Section
func TestMarkdownFormatter_OrphanedSection(t *testing.T) {
	r := &spool.Report{
		Orphaned: []spool.OrphanedRef{
			{ID: "AUTH-99-1", File: "auth_test.go", Line: 5},
		},
	}

	data, err := Markdown{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "AUTH-99-1") {
		t.Error("output should contain orphaned ID")
	}
	if !strings.Contains(out, "auth_test.go") {
		t.Error("output should contain file path")
	}
}

// REPORTER-5-4: Excluded Reqs in Table
func TestMarkdownFormatter_ExcludedReqs(t *testing.T) {
	r := &spool.Report{
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 1, ExcludedRequirements: 2},
		},
	}

	data, err := Markdown{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "2") {
		t.Error("output should show excluded count")
	}
}
