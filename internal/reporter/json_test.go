package reporter

import (
	"encoding/json"
	"testing"

	spool "github.com/mwames/Spool"
)

// REPORTER-4-1: Valid JSON
func TestJSONFormatter_ValidJSON(t *testing.T) {
	r := &spool.Report{
		TotalRequirements: 2,
		TotalACs:          3,
		LinkedACs:         1,
		UntestedACs:       2,
		OrphanedCount:     1,
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 2, ACs: 3, LinkedACs: 1, UntestedACs: 2},
		},
		Untested: []spool.UntestedAC{
			{ID: "AUTH-1-1", Description: "login", Feature: "AUTH"},
		},
		Orphaned: []spool.OrphanedRef{
			{ID: "AUTH-99-1", File: "test.go", Line: 5},
		},
	}

	data, err := JSON{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["totalRequirements"] != float64(2) {
		t.Errorf("totalRequirements = %v, want 2", parsed["totalRequirements"])
	}
	if parsed["totalACs"] != float64(3) {
		t.Errorf("totalACs = %v, want 3", parsed["totalACs"])
	}
}

// REPORTER-4-2: camelCase field names
func TestJSONFormatter_CamelCase(t *testing.T) {
	r := &spool.Report{
		TotalRequirements: 1,
		Features: []spool.FeatureReport{
			{Name: "AUTH", ExcludedRequirements: 1},
		},
	}

	data, err := JSON{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	// Check camelCase keys exist
	for _, key := range []string{"totalRequirements", "totalACs", "linkedACs", "untestedACs", "orphanedCount", "features"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing camelCase key %q", key)
		}
	}

	// Check nested feature uses camelCase
	features := parsed["features"].([]any)
	if len(features) > 0 {
		feat := features[0].(map[string]any)
		if _, ok := feat["excludedRequirements"]; !ok {
			t.Error("missing camelCase key excludedRequirements in feature")
		}
	}
}
