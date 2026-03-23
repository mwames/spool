package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeReqFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const validReqYAML = `
feature: AUTH

requirements:
  - id: AUTH-1
    title: Login
    description: Users must be able to log in.
    status: active
    deciders:
      - Project Lead
    consulted: []
    date: 2026-03-23
    rationale: Core feature.
    superseded_by:
    acceptance_criteria:
      - id: AUTH-1-1
        title: Valid Credentials
        description: Given valid credentials, login succeeds.
      - id: AUTH-1-2
        title: Invalid Credentials
        description: Given invalid credentials, login fails.
  - id: AUTH-2
    title: Logout
    description: Users must be able to log out.
    status: active
    deciders:
      - Project Lead
    consulted: []
    date: 2026-03-23
    rationale: Core feature.
    superseded_by:
    acceptance_criteria:
      - id: AUTH-2-1
        title: Session Cleared
        description: Session is cleared on logout.
`

// PARSER-1-1: Valid .req file → populated ReqFile
func TestParseFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.req")
	writeReqFile(t, path, validReqYAML)

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rf.Feature != "AUTH" {
		t.Errorf("Feature = %q, want %q", rf.Feature, "AUTH")
	}
	if len(rf.Requirements) != 2 {
		t.Fatalf("len(Requirements) = %d, want 2", len(rf.Requirements))
	}

	r1 := rf.Requirements[0]
	if r1.ID != "AUTH-1" {
		t.Errorf("Requirements[0].ID = %q, want %q", r1.ID, "AUTH-1")
	}
	if r1.Title != "Login" {
		t.Errorf("Requirements[0].Title = %q, want %q", r1.Title, "Login")
	}
	if len(r1.AcceptanceCriteria) != 2 {
		t.Fatalf("len(Requirements[0].ACs) = %d, want 2", len(r1.AcceptanceCriteria))
	}
	if r1.AcceptanceCriteria[0].ID != "AUTH-1-1" {
		t.Errorf("AC[0].ID = %q, want %q", r1.AcceptanceCriteria[0].ID, "AUTH-1-1")
	}
	if r1.AcceptanceCriteria[0].Title != "Valid Credentials" {
		t.Errorf("AC[0].Title = %q, want %q", r1.AcceptanceCriteria[0].Title, "Valid Credentials")
	}

	r2 := rf.Requirements[1]
	if r2.ID != "AUTH-2" {
		t.Errorf("Requirements[1].ID = %q, want %q", r2.ID, "AUTH-2")
	}
}

// PARSER-1-2: Zero requirements → empty slice
func TestParseFile_ZeroRequirements(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.req")
	writeReqFile(t, path, "feature: EMPTY\nrequirements: []\n")

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rf.Requirements) != 0 {
		t.Errorf("len(Requirements) = %d, want 0", len(rf.Requirements))
	}
}

// PARSER-1-3: Requirement with zero ACs → empty ACs slice
func TestParseFile_ZeroACs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noac.req")
	writeReqFile(t, path, `
feature: NOAC
requirements:
  - id: NOAC-1
    title: Something
    description: A requirement with no ACs.
    status: active
    acceptance_criteria: []
`)

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rf.Requirements) != 1 {
		t.Fatalf("len(Requirements) = %d, want 1", len(rf.Requirements))
	}
	if len(rf.Requirements[0].AcceptanceCriteria) != 0 {
		t.Errorf("len(ACs) = %d, want 0", len(rf.Requirements[0].AcceptanceCriteria))
	}
}

// PARSER-1-4: SourceFile populated with absolute path
func TestParseFile_SourceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src.req")
	writeReqFile(t, path, "feature: SRC\nrequirements: []\n")

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(rf.SourceFile) {
		t.Errorf("SourceFile = %q, want absolute path", rf.SourceFile)
	}
	if rf.SourceFile != path {
		t.Errorf("SourceFile = %q, want %q", rf.SourceFile, path)
	}
}

// PARSER-2-1: Lowercase ID normalized to uppercase
func TestParseFile_NormalizeIDUppercase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lower.req")
	writeReqFile(t, path, `
feature: lower
requirements:
  - id: config-1
    title: Test
    description: Test.
    status: active
    acceptance_criteria:
      - id: config-1-1
        description: Test AC.
`)

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rf.Requirements[0].ID != "CONFIG-1" {
		t.Errorf("ID = %q, want %q", rf.Requirements[0].ID, "CONFIG-1")
	}
	if rf.Requirements[0].AcceptanceCriteria[0].ID != "CONFIG-1-1" {
		t.Errorf("AC ID = %q, want %q", rf.Requirements[0].AcceptanceCriteria[0].ID, "CONFIG-1-1")
	}
}

// PARSER-2-2: Leading zeros stripped
func TestParseFile_NormalizeIDLeadingZeros(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zeros.req")
	writeReqFile(t, path, `
feature: ZEROS
requirements:
  - id: PARSER-02
    title: Test
    description: Test.
    status: active
    acceptance_criteria:
      - id: PARSER-02-01
        description: Test AC.
`)

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rf.Requirements[0].ID != "PARSER-2" {
		t.Errorf("ID = %q, want %q", rf.Requirements[0].ID, "PARSER-2")
	}
	if rf.Requirements[0].AcceptanceCriteria[0].ID != "PARSER-2-1" {
		t.Errorf("AC ID = %q, want %q", rf.Requirements[0].AcceptanceCriteria[0].ID, "PARSER-2-1")
	}
}

// PARSER-2-3: Canonical ID unchanged
func TestParseFile_NormalizeIDCanonical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "canonical.req")
	writeReqFile(t, path, `
feature: CANON
requirements:
  - id: CANON-1
    title: Test
    description: Test.
    status: active
    acceptance_criteria:
      - id: CANON-1-1
        description: Test AC.
`)

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rf.Requirements[0].ID != "CANON-1" {
		t.Errorf("ID = %q, want %q", rf.Requirements[0].ID, "CANON-1")
	}
	if rf.Requirements[0].AcceptanceCriteria[0].ID != "CANON-1-1" {
		t.Errorf("AC ID = %q, want %q", rf.Requirements[0].AcceptanceCriteria[0].ID, "CANON-1-1")
	}
}

// PARSER-3-1: Nested .req files all parsed
func TestParseDir_Nested(t *testing.T) {
	dir := t.TempDir()
	writeReqFile(t, filepath.Join(dir, "a.req"), "feature: A\nrequirements: []\n")
	writeReqFile(t, filepath.Join(dir, "sub", "b.req"), "feature: B\nrequirements: []\n")
	writeReqFile(t, filepath.Join(dir, "sub", "deep", "c.req"), "feature: C\nrequirements: []\n")

	result, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 3 {
		t.Errorf("len(Files) = %d, want 3", len(result.Files))
	}
	if len(result.Errors) != 0 {
		t.Errorf("len(Errors) = %d, want 0", len(result.Errors))
	}
}

// PARSER-3-2: Non-.req files ignored
func TestParseDir_IgnoresNonReq(t *testing.T) {
	dir := t.TempDir()
	writeReqFile(t, filepath.Join(dir, "a.req"), "feature: A\nrequirements: []\n")
	writeReqFile(t, filepath.Join(dir, "readme.md"), "# readme\n")
	writeReqFile(t, filepath.Join(dir, "notes.txt"), "some notes\n")

	result, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 1 {
		t.Errorf("len(Files) = %d, want 1", len(result.Files))
	}
}

// PARSER-3-3: Missing dir → error
func TestParseDir_MissingDir(t *testing.T) {
	_, err := ParseDir("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

// PARSER-3-4: Empty dir → empty slice
func TestParseDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("len(Files) = %d, want 0", len(result.Files))
	}
	if len(result.Errors) != 0 {
		t.Errorf("len(Errors) = %d, want 0", len(result.Errors))
	}
}

// PARSER-4-1: Invalid YAML → error for that file, others continue
func TestParseDir_InvalidYAMLContinues(t *testing.T) {
	dir := t.TempDir()
	writeReqFile(t, filepath.Join(dir, "good.req"), "feature: GOOD\nrequirements: []\n")
	writeReqFile(t, filepath.Join(dir, "bad.req"), "feature: [invalid\n")

	result, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 1 {
		t.Errorf("len(Files) = %d, want 1", len(result.Files))
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
	}
	if !strings.Contains(result.Errors[0].Path, "bad.req") {
		t.Errorf("error path = %q, want to contain bad.req", result.Errors[0].Path)
	}
}

// PARSER-4-2: Malformed ID → warning recorded, other reqs still parsed
func TestParseFile_MalformedID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.req")
	writeReqFile(t, path, `
feature: MAL
requirements:
  - id: "not an id at all"
    title: Bad
    description: Bad ID.
    status: active
    acceptance_criteria: []
  - id: MAL-1
    title: Good
    description: Good ID.
    status: active
    acceptance_criteria: []
`)

	rf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rf.Requirements) != 2 {
		t.Fatalf("len(Requirements) = %d, want 2", len(rf.Requirements))
	}
	// Good requirement is normalized
	if rf.Requirements[1].ID != "MAL-1" {
		t.Errorf("Requirements[1].ID = %q, want %q", rf.Requirements[1].ID, "MAL-1")
	}
	// Malformed ID kept as-is, warning recorded
	if rf.Requirements[0].ID != "not an id at all" {
		t.Errorf("Requirements[0].ID = %q, want original", rf.Requirements[0].ID)
	}
	if len(rf.Warnings) == 0 {
		t.Error("expected warnings for malformed ID")
	}
}
