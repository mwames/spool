package reporter

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

func buildIndex(reqFiles []*parser.ReqFile, scanResults []scanner.FileResult) *index.Index {
	return index.Build(reqFiles, scanResults)
}

func reqFile(feature string, reqs ...parser.Requirement) *parser.ReqFile {
	return &parser.ReqFile{
		Feature:      feature,
		Requirements: reqs,
		SourceFile:   "/path/" + feature + ".req",
	}
}

func req(id, status string, acs ...parser.AcceptanceCriterion) parser.Requirement {
	return parser.Requirement{ID: id, Title: id, Status: status, AcceptanceCriteria: acs}
}

func ac(id, desc string) parser.AcceptanceCriterion {
	return parser.AcceptanceCriterion{ID: id, Title: id, Description: desc}
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

// --- REPORTER-1: Report Generation ---

// REPORTER-1-1: Accurate Totals
func TestGenerate_AccurateTotals(t *testing.T) {
	idx := buildIndex(
		[]*parser.ReqFile{
			reqFile("AUTH",
				req("AUTH-1", "active", ac("AUTH-1-1", "login"), ac("AUTH-1-2", "logout")),
				req("AUTH-2", "active", ac("AUTH-2-1", "session")),
			),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go",
				mapping("TestLogin", 5, "AUTH-1-1"),
				mapping("TestSession", 10, "AUTH-2-1"),
			),
		},
	)

	r := Generate(idx)
	if r.TotalRequirements != 2 {
		t.Errorf("TotalRequirements = %d, want 2", r.TotalRequirements)
	}
	if r.TotalACs != 3 {
		t.Errorf("TotalACs = %d, want 3", r.TotalACs)
	}
	if r.LinkedACs != 2 {
		t.Errorf("LinkedACs = %d, want 2", r.LinkedACs)
	}
	if r.UntestedACs != 1 {
		t.Errorf("UntestedACs = %d, want 1", r.UntestedACs)
	}
	if r.OrphanedCount != 0 {
		t.Errorf("OrphanedCount = %d, want 0", r.OrphanedCount)
	}
}

// REPORTER-1-2: Per-Feature Totals
func TestGenerate_PerFeatureTotals(t *testing.T) {
	idx := buildIndex(
		[]*parser.ReqFile{
			reqFile("AUTH",
				req("AUTH-1", "active", ac("AUTH-1-1", "login")),
				req("AUTH-2", "superseded", ac("AUTH-2-1", "old")),
			),
		},
		nil,
	)

	r := Generate(idx)
	if len(r.Features) != 1 {
		t.Fatalf("len(Features) = %d, want 1", len(r.Features))
	}
	f := r.Features[0]
	if f.Name != "AUTH" {
		t.Errorf("Name = %q, want AUTH", f.Name)
	}
	if f.Requirements != 1 {
		t.Errorf("Requirements = %d, want 1 (active only)", f.Requirements)
	}
	if f.ExcludedRequirements != 1 {
		t.Errorf("ExcludedRequirements = %d, want 1", f.ExcludedRequirements)
	}
	if f.ACs != 1 {
		t.Errorf("ACs = %d, want 1 (under active reqs)", f.ACs)
	}
}

// REPORTER-1-3: Untested ACs List
func TestGenerate_UntestedACsList(t *testing.T) {
	idx := buildIndex(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", "must login"))),
		},
		nil,
	)

	r := Generate(idx)
	if len(r.Untested) != 1 {
		t.Fatalf("len(Untested) = %d, want 1", len(r.Untested))
	}
	if r.Untested[0].ID != "AUTH-1-1" {
		t.Errorf("ID = %q, want AUTH-1-1", r.Untested[0].ID)
	}
	if r.Untested[0].Description != "must login" {
		t.Errorf("Description = %q, want %q", r.Untested[0].Description, "must login")
	}
	if r.Untested[0].Feature != "AUTH" {
		t.Errorf("Feature = %q, want AUTH", r.Untested[0].Feature)
	}
}

// REPORTER-1-4: Orphaned Annotations List
func TestGenerate_OrphanedList(t *testing.T) {
	idx := buildIndex(
		[]*parser.ReqFile{
			reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", "login"))),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go", mapping("TestBad", 5, "AUTH-99-1")),
		},
	)

	r := Generate(idx)
	if len(r.Orphaned) != 1 {
		t.Fatalf("len(Orphaned) = %d, want 1", len(r.Orphaned))
	}
	if r.Orphaned[0].ID != "AUTH-99-1" {
		t.Errorf("ID = %q, want AUTH-99-1", r.Orphaned[0].ID)
	}
	if r.Orphaned[0].File != "auth_test.go" {
		t.Errorf("File = %q, want auth_test.go", r.Orphaned[0].File)
	}
}

// REPORTER-1-5: Empty Index
func TestGenerate_Empty(t *testing.T) {
	idx := buildIndex(nil, nil)
	r := Generate(idx)

	if r.TotalRequirements != 0 || r.TotalACs != 0 || r.LinkedACs != 0 || r.UntestedACs != 0 || r.OrphanedCount != 0 {
		t.Errorf("expected all zeros, got %+v", r)
	}
	if len(r.Features) != 0 {
		t.Errorf("len(Features) = %d, want 0", len(r.Features))
	}
}

// --- REPORTER-2: Formatter Interface ---

type mockFormatter struct{}

func (mockFormatter) Format(report *spool.Report) ([]byte, error) {
	return []byte("mock"), nil
}

// REPORTER-2-1: Minimal Interface Contract
// REPORTER-2-2: Third-Party Formatter
func TestFormatter_MockCompliance(t *testing.T) {
	var f spool.Formatter = mockFormatter{}
	r := &spool.Report{TotalRequirements: 1}
	data, err := f.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "mock" {
		t.Errorf("got %q, want %q", string(data), "mock")
	}
}

// --- REPORTER-6: Output ---

// REPORTER-6-1: Write to stdout
func TestWrite_Stdout(t *testing.T) {
	var buf bytes.Buffer
	err := WriteTo([]byte("hello\n"), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "hello\n" {
		t.Errorf("got %q", buf.String())
	}
}

// REPORTER-6-2: Write to file
func TestWrite_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.txt")

	err := Write([]byte("report content"), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(data) != "report content" {
		t.Errorf("got %q", string(data))
	}
}

// REPORTER-6-3: Unwritable file
func TestWrite_Unwritable(t *testing.T) {
	err := Write([]byte("data"), "/nonexistent/dir/report.txt")
	if err == nil {
		t.Error("expected error for unwritable path")
	}
}
