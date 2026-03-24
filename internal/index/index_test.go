package index

import (
	"strings"
	"testing"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

func reqFile(feature string, reqs ...parser.Requirement) *parser.ReqFile {
	return &parser.ReqFile{
		Feature:      feature,
		Requirements: reqs,
		SourceFile:   "/path/" + strings.ToLower(feature) + ".req",
	}
}

func req(id, status string, acs ...parser.AcceptanceCriterion) parser.Requirement {
	return parser.Requirement{
		ID:                 id,
		Title:              id,
		Status:             status,
		AcceptanceCriteria: acs,
	}
}

func ac(id string) parser.AcceptanceCriterion {
	return parser.AcceptanceCriterion{ID: id, Title: id, Description: "test"}
}

func scanResult(path string, mappings ...scanner.TestMapping) scanner.FileResult {
	return scanner.FileResult{Path: path, Mappings: mappings}
}

func mapping(funcName string, line int, annIDs ...string) scanner.TestMapping {
	var anns []scanner.Annotation
	for _, id := range annIDs {
		anns = append(anns, scanner.Annotation{ID: id, Line: line - 1})
	}
	return scanner.TestMapping{
		Function:    spool.TestFunction{Name: funcName, Line: line},
		Annotations: anns,
	}
}

// --- INDEX-1: Index Construction ---

// INDEX-1-1: All reqs and ACs present
func TestBuild_AllReqsAndACsPresent(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH",
				req("AUTH-1", "active", ac("AUTH-1-1"), ac("AUTH-1-2")),
				req("AUTH-2", "active", ac("AUTH-2-1")),
			),
		},
		nil,
	)

	if r := idx.Requirement("AUTH-1"); r == nil {
		t.Error("AUTH-1 not found")
	}
	if r := idx.Requirement("AUTH-2"); r == nil {
		t.Error("AUTH-2 not found")
	}
	if a := idx.AC("AUTH-1-1"); a == nil {
		t.Error("AUTH-1-1 not found")
	}
	if a := idx.AC("AUTH-1-2"); a == nil {
		t.Error("AUTH-1-2 not found")
	}
	if a := idx.AC("AUTH-2-1"); a == nil {
		t.Error("AUTH-2-1 not found")
	}
}

// INDEX-1-2: ACs linked to test functions
func TestBuild_ACsLinkedToTests(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go", mapping("TestLogin", 5, "AUTH-1-1")),
		},
	)

	a := idx.AC("AUTH-1-1")
	if a == nil {
		t.Fatal("AUTH-1-1 not found")
	}
	if len(a.Tests) != 1 {
		t.Fatalf("len(Tests) = %d, want 1", len(a.Tests))
	}
	if a.Tests[0].Function.Name != "TestLogin" {
		t.Errorf("Function.Name = %q, want %q", a.Tests[0].Function.Name, "TestLogin")
	}
	if a.Tests[0].File != "auth_test.go" {
		t.Errorf("File = %q, want %q", a.Tests[0].File, "auth_test.go")
	}
}

// INDEX-1-3: Feature hierarchy
func TestBuild_FeatureHierarchy(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
			reqFile("CONFIG", req("CONFIG-1", "active", ac("CONFIG-1-1"))),
		},
		nil,
	)

	f := idx.Feature("AUTH")
	if f == nil {
		t.Fatal("AUTH feature not found")
	}
	if len(f.Requirements) != 1 {
		t.Errorf("AUTH has %d reqs, want 1", len(f.Requirements))
	}
	if f.Requirements[0].ID != "AUTH-1" {
		t.Errorf("first req ID = %q, want AUTH-1", f.Requirements[0].ID)
	}

	f2 := idx.Feature("CONFIG")
	if f2 == nil {
		t.Fatal("CONFIG feature not found")
	}
}

// INDEX-1-4: Empty inputs → valid empty index
func TestBuild_Empty(t *testing.T) {
	idx := Build(nil, nil)
	if len(idx.Features()) != 0 {
		t.Errorf("Features = %d, want 0", len(idx.Features()))
	}
	if len(idx.UntestedACs()) != 0 {
		t.Errorf("UntestedACs = %d, want 0", len(idx.UntestedACs()))
	}
	if len(idx.OrphanedAnnotations()) != 0 {
		t.Errorf("Orphaned = %d, want 0", len(idx.OrphanedAnnotations()))
	}
	if len(idx.Errors()) != 0 {
		t.Errorf("Errors = %d, want 0", len(idx.Errors()))
	}
}

// --- INDEX-2: ID Uniqueness ---

// INDEX-2-1: Duplicate req ID
func TestBuild_DuplicateReqID(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active")),
			reqFile("AUTH", req("AUTH-1", "active")),
		},
		nil,
	)

	errs := idx.Errors()
	if len(errs) == 0 {
		t.Fatal("expected error for duplicate req ID")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "AUTH-1") {
			found = true
		}
	}
	if !found {
		t.Errorf("error should mention AUTH-1, got %v", errs)
	}
}

// INDEX-2-2: Duplicate AC ID
func TestBuild_DuplicateACID(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
			reqFile("AUTH", req("AUTH-2", "active", ac("AUTH-1-1"))),
		},
		nil,
	)

	errs := idx.Errors()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "AUTH-1-1") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error mentioning AUTH-1-1, got %v", errs)
	}
}

// INDEX-2-3: All unique → no errors
func TestBuild_AllUnique(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
			reqFile("CONFIG", req("CONFIG-1", "active", ac("CONFIG-1-1"))),
		},
		nil,
	)

	if len(idx.Errors()) != 0 {
		t.Errorf("expected no errors, got %v", idx.Errors())
	}
}

// --- INDEX-3: Feature Prefix Validation ---

// INDEX-3-1: Mismatched prefix
func TestBuild_MismatchedPrefix(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("CONFIG-1", "active")),
		},
		nil,
	)

	errs := idx.Errors()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "CONFIG-1") && strings.Contains(e.Message, "AUTH") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected prefix mismatch error, got %v", errs)
	}
}

// INDEX-3-2: Matching prefixes → no errors
func TestBuild_MatchingPrefixes(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
		},
		nil,
	)

	if len(idx.Errors()) != 0 {
		t.Errorf("expected no errors, got %v", idx.Errors())
	}
}

// --- INDEX-4: Untested AC Detection ---

// INDEX-4-1: Active, untested → in UntestedACs
func TestUntestedACs_ActiveUntested(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
		},
		nil,
	)

	untested := idx.UntestedACs()
	if len(untested) != 1 {
		t.Fatalf("len(UntestedACs) = %d, want 1", len(untested))
	}
	if untested[0].ID != "AUTH-1-1" {
		t.Errorf("ID = %q, want AUTH-1-1", untested[0].ID)
	}
}

// INDEX-4-2: Active, tested → not in UntestedACs
func TestUntestedACs_ActiveTested(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go", mapping("TestLogin", 5, "AUTH-1-1")),
		},
	)

	if len(idx.UntestedACs()) != 0 {
		t.Errorf("len(UntestedACs) = %d, want 0", len(idx.UntestedACs()))
	}
}

// INDEX-4-3: Non-active, untested → not in UntestedACs
func TestUntestedACs_NonActiveUntested(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "superseded", ac("AUTH-1-1"))),
		},
		nil,
	)

	if len(idx.UntestedACs()) != 0 {
		t.Errorf("len(UntestedACs) = %d, want 0", len(idx.UntestedACs()))
	}
}

// INDEX-4-4: Superseded req still navigable
func TestIndex_SupersededNavigable(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "superseded", ac("AUTH-1-1"))),
		},
		nil,
	)

	r := idx.Requirement("AUTH-1")
	if r == nil {
		t.Fatal("superseded req should be navigable")
	}
	a := idx.AC("AUTH-1-1")
	if a == nil {
		t.Fatal("AC under superseded req should be navigable")
	}
}

// --- INDEX-5: Orphaned Annotation Detection ---

// INDEX-5-1: Unknown ID → orphaned
func TestBuild_OrphanedAnnotation(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go", mapping("TestLogin", 5, "AUTH-99-1")),
		},
	)

	orphaned := idx.OrphanedAnnotations()
	if len(orphaned) != 1 {
		t.Fatalf("len(Orphaned) = %d, want 1", len(orphaned))
	}
	if orphaned[0].ID != "AUTH-99-1" {
		t.Errorf("ID = %q, want AUTH-99-1", orphaned[0].ID)
	}
}

// INDEX-5-2: All valid → empty orphaned
func TestBuild_NoOrphaned(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go", mapping("TestLogin", 5, "AUTH-1-1")),
		},
	)

	if len(idx.OrphanedAnnotations()) != 0 {
		t.Errorf("len(Orphaned) = %d, want 0", len(idx.OrphanedAnnotations()))
	}
}

// --- INDEX-6: Querying ---

// INDEX-6-1: Query by feature
func TestQuery_ByFeature(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active"), req("AUTH-2", "active")),
		},
		nil,
	)

	f := idx.Feature("AUTH")
	if f == nil {
		t.Fatal("AUTH not found")
	}
	if len(f.Requirements) != 2 {
		t.Errorf("len(Requirements) = %d, want 2", len(f.Requirements))
	}
}

// INDEX-6-2: Query by req ID
func TestQuery_ByReqID(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"), ac("AUTH-1-2"))),
		},
		nil,
	)

	r := idx.Requirement("AUTH-1")
	if r == nil {
		t.Fatal("AUTH-1 not found")
	}
	if len(r.ACs) != 2 {
		t.Errorf("len(ACs) = %d, want 2", len(r.ACs))
	}
}

// INDEX-6-3: Query by AC ID
func TestQuery_ByACID(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1"))),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go", mapping("TestLogin", 5, "AUTH-1-1")),
		},
	)

	a := idx.AC("AUTH-1-1")
	if a == nil {
		t.Fatal("AUTH-1-1 not found")
	}
	if len(a.Tests) != 1 {
		t.Errorf("len(Tests) = %d, want 1", len(a.Tests))
	}
}

// INDEX-6-4: Unknown ID → nil
func TestQuery_UnknownID(t *testing.T) {
	idx := Build(nil, nil)
	if idx.Feature("NOPE") != nil {
		t.Error("expected nil for unknown feature")
	}
	if idx.Requirement("NOPE-1") != nil {
		t.Error("expected nil for unknown req")
	}
	if idx.AC("NOPE-1-1") != nil {
		t.Error("expected nil for unknown AC")
	}
}

// INDEX-6-5: All features
func TestQuery_AllFeatures(t *testing.T) {
	idx := Build(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active")),
			reqFile("CONFIG", req("CONFIG-1", "active")),
		},
		nil,
	)

	features := idx.Features()
	if len(features) != 2 {
		t.Errorf("len(Features) = %d, want 2", len(features))
	}
}
