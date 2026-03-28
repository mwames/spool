package lsp

import (
	"os"
	"path/filepath"
	"testing"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

func buildTestIndex(reqFiles []*parser.ReqFile, scanResults []scanner.FileResult) *index.Index {
	return index.Build(reqFiles, scanResults)
}

func reqFile(feature string, reqs ...parser.Requirement) *parser.ReqFile {
	return &parser.ReqFile{
		Feature:      feature,
		Requirements: reqs,
		SourceFile:   "/reqs/" + feature + ".req",
	}
}

func req(id, status string, acs ...parser.AcceptanceCriterion) parser.Requirement {
	return parser.Requirement{
		ID:                 id,
		Title:              id,
		Status:             status,
		AcceptanceCriteria: acs,
		Line:               1,
	}
}

func ac(id string, line int) parser.AcceptanceCriterion {
	return parser.AcceptanceCriterion{ID: id, Title: id, Description: "test", Line: line}
}

func scanResult(path string, mappings ...scanner.TestMapping) scanner.FileResult {
	return scanner.FileResult{Path: path, Mappings: mappings}
}

func mapping(funcName string, funcLine int, annIDs ...string) scanner.TestMapping {
	var anns []scanner.Annotation
	for _, id := range annIDs {
		anns = append(anns, scanner.Annotation{ID: id, Line: funcLine - 1})
	}
	return scanner.TestMapping{
		Function:    spool.TestFunction{Name: funcName, Line: funcLine},
		Annotations: anns,
	}
}

// --- extractAnnotationID ---

func TestExtractAnnotationID_SlashSlash(t *testing.T) {
	got := extractAnnotationID("// AUTH-1-1")
	if got != "AUTH-1-1" {
		t.Errorf("got %q, want AUTH-1-1", got)
	}
}

func TestExtractAnnotationID_Hash(t *testing.T) {
	got := extractAnnotationID("# AUTH-1-1")
	if got != "AUTH-1-1" {
		t.Errorf("got %q, want AUTH-1-1", got)
	}
}

func TestExtractAnnotationID_WithTitle(t *testing.T) {
	got := extractAnnotationID("// AUTH-1-1: Valid Credentials")
	if got != "AUTH-1-1" {
		t.Errorf("got %q, want AUTH-1-1", got)
	}
}

func TestExtractAnnotationID_Normalizes(t *testing.T) {
	got := extractAnnotationID("// auth-01-02")
	if got != "AUTH-1-2" {
		t.Errorf("got %q, want AUTH-1-2", got)
	}
}

func TestExtractAnnotationID_NotAnnotation(t *testing.T) {
	got := extractAnnotationID("func TestSomething(t *testing.T) {")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractAnnotationID_EmptyLine(t *testing.T) {
	got := extractAnnotationID("")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- extractReqID ---

func TestExtractReqID_ACID(t *testing.T) {
	got := extractReqID("      - id: CONFIG-1-1")
	if got != "CONFIG-1-1" {
		t.Errorf("got %q, want CONFIG-1-1", got)
	}
}

func TestExtractReqID_ReqID(t *testing.T) {
	got := extractReqID("  - id: CONFIG-1")
	if got != "CONFIG-1" {
		t.Errorf("got %q, want CONFIG-1", got)
	}
}

func TestExtractReqID_NoID(t *testing.T) {
	got := extractReqID("    title: Some Title")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractReqID_Normalizes(t *testing.T) {
	got := extractReqID("      - id: config-01-02")
	if got != "CONFIG-1-2" {
		t.Errorf("got %q, want CONFIG-1-2", got)
	}
}

// --- resolve: test annotation → AC definition ---

// LSP-1-1: Annotation Resolves to AC Definition
func TestResolveFromTest_AnnotationToAC(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "auth_test.go")
	os.WriteFile(testFile, []byte("// AUTH-1-1\nfunc TestLogin(t *testing.T) {}\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		[]scanner.FileResult{scanResult(testFile, mapping("TestLogin", 2, "AUTH-1-1"))},
	)

	locs := resolve(idx, testFile, 0) // line 0 = "// AUTH-1-1"
	if len(locs) != 1 {
		t.Fatalf("len(locs) = %d, want 1", len(locs))
	}
	if locs[0].Range.Start.Line != 9 { // AC line 10 → 0-based 9
		t.Errorf("Start.Line = %d, want 9", locs[0].Range.Start.Line)
	}
}

// LSP-1-2: Unknown Annotation Returns Empty
func TestResolveFromTest_UnknownAnnotation(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "auth_test.go")
	os.WriteFile(testFile, []byte("// AUTH-99-1\nfunc TestLogin(t *testing.T) {}\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	locs := resolve(idx, testFile, 0)
	if len(locs) != 0 {
		t.Errorf("len(locs) = %d, want 0", len(locs))
	}
}

// LSP-1-3: Non-Annotation Line Returns Empty
func TestResolveFromTest_NonAnnotationLine(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "auth_test.go")
	os.WriteFile(testFile, []byte("// AUTH-1-1\nfunc TestLogin(t *testing.T) {}\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	locs := resolve(idx, testFile, 1) // line 1 = func line
	if len(locs) != 0 {
		t.Errorf("len(locs) = %d, want 0", len(locs))
	}
}

// --- resolve: AC → tests ---

// LSP-2-1: AC Resolves to Test Locations
func TestResolveFromReq_ACToTests(t *testing.T) {
	dir := t.TempDir()
	reqPath := filepath.Join(dir, "auth.req")
	os.WriteFile(reqPath, []byte("feature: AUTH\nrequirements:\n  - id: AUTH-1\n    status: active\n    acceptance_criteria:\n      - id: AUTH-1-1\n        title: Test\n"), 0644)

	testFile := "/tests/auth_test.go"
	idx := buildTestIndex(
		[]*parser.ReqFile{{
			Feature:    "AUTH",
			SourceFile: reqPath,
			Requirements: []parser.Requirement{{
				ID:     "AUTH-1",
				Status: "active",
				Line:   3,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "AUTH-1-1", Title: "Test", Line: 6},
				},
			}},
		}},
		[]scanner.FileResult{scanResult(testFile, mapping("TestLogin", 5, "AUTH-1-1"))},
	)

	locs := resolve(idx, reqPath, 5) // line 5 (0-based) = "      - id: AUTH-1-1"
	if len(locs) != 1 {
		t.Fatalf("len(locs) = %d, want 1", len(locs))
	}
	if locs[0].Range.Start.Line != 4 { // test func line 5 → 0-based 4
		t.Errorf("Start.Line = %d, want 4", locs[0].Range.Start.Line)
	}
}

// LSP-2-2: Untested AC Returns Empty
func TestResolveFromReq_UntestedAC(t *testing.T) {
	dir := t.TempDir()
	reqPath := filepath.Join(dir, "auth.req")
	os.WriteFile(reqPath, []byte("feature: AUTH\nrequirements:\n  - id: AUTH-1\n    status: active\n    acceptance_criteria:\n      - id: AUTH-1-1\n        title: Test\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{{
			Feature:    "AUTH",
			SourceFile: reqPath,
			Requirements: []parser.Requirement{{
				ID:     "AUTH-1",
				Status: "active",
				Line:   3,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "AUTH-1-1", Title: "Test", Line: 6},
				},
			}},
		}},
		nil,
	)

	locs := resolve(idx, reqPath, 5) // "      - id: AUTH-1-1"
	if len(locs) != 0 {
		t.Errorf("len(locs) = %d, want 0", len(locs))
	}
}

// LSP-2-3: Non-ID Line Returns Empty
func TestResolveFromReq_NonIDLine(t *testing.T) {
	dir := t.TempDir()
	reqPath := filepath.Join(dir, "auth.req")
	os.WriteFile(reqPath, []byte("feature: AUTH\nrequirements:\n  - id: AUTH-1\n    title: Login\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	locs := resolve(idx, reqPath, 3) // "    title: Login"
	if len(locs) != 0 {
		t.Errorf("len(locs) = %d, want 0", len(locs))
	}
}

// LSP-2-1 (multiple tests): AC with multiple covering tests
func TestResolveFromReq_ACToMultipleTests(t *testing.T) {
	dir := t.TempDir()
	reqPath := filepath.Join(dir, "auth.req")
	os.WriteFile(reqPath, []byte("feature: AUTH\nrequirements:\n  - id: AUTH-1\n    status: active\n    acceptance_criteria:\n      - id: AUTH-1-1\n        title: Test\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{{
			Feature:    "AUTH",
			SourceFile: reqPath,
			Requirements: []parser.Requirement{{
				ID:     "AUTH-1",
				Status: "active",
				Line:   3,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "AUTH-1-1", Title: "Test", Line: 6},
				},
			}},
		}},
		[]scanner.FileResult{
			scanResult("/tests/a_test.go", mapping("TestLoginBasic", 5, "AUTH-1-1")),
			scanResult("/tests/b_test.go", mapping("TestLoginEdge", 10, "AUTH-1-1")),
		},
	)

	locs := resolve(idx, reqPath, 5) // "      - id: AUTH-1-1"
	if len(locs) != 2 {
		t.Fatalf("len(locs) = %d, want 2", len(locs))
	}
}

// --- collectLinks: .req file ---

// LSP-4-1
func TestCollectLinks_ReqFileTestedAC(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 6)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 5, "AUTH-1-1"))},
	)

	lines := []string{
		"feature: AUTH",
		"requirements:",
		"  - id: AUTH-1",
		"    status: active",
		"    acceptance_criteria:",
		"      - id: AUTH-1-1",
	}

	links := collectReqLinks(idx, lines)
	if len(links) != 2 {
		t.Fatalf("len(links) = %d, want 2 (req + AC)", len(links))
	}

	// The AC link should cover "AUTH-1-1" on line 5.
	acLink := links[1]
	if acLink.Range.Start.Line != 5 {
		t.Errorf("AC link line = %d, want 5", acLink.Range.Start.Line)
	}
	if acLink.Tooltip != "Go to test: TestLogin" {
		t.Errorf("tooltip = %q, want %q", acLink.Tooltip, "Go to test: TestLogin")
	}
}

// LSP-4-2
func TestCollectLinks_ReqFileUntestedAC(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 6)))},
		nil,
	)

	lines := []string{
		"feature: AUTH",
		"requirements:",
		"  - id: AUTH-1",
		"    status: active",
		"    acceptance_criteria:",
		"      - id: AUTH-1-1",
	}

	links := collectReqLinks(idx, lines)
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0 (untested)", len(links))
	}
}

// --- collectLinks: test file ---

// LSP-4-3
func TestCollectLinks_TestFileKnownAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 2, "AUTH-1-1"))},
	)

	lines := []string{
		"// AUTH-1-1",
		"func TestLogin(t *testing.T) {}",
	}

	links := collectTestLinks(idx, lines)
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	if links[0].Range.Start.Line != 0 {
		t.Errorf("link line = %d, want 0", links[0].Range.Start.Line)
	}
	if links[0].Range.Start.Character != 3 {
		t.Errorf("link start char = %d, want 3", links[0].Range.Start.Character)
	}
	if links[0].Range.End.Character != 11 {
		t.Errorf("link end char = %d, want 11", links[0].Range.End.Character)
	}
}

// LSP-4-4
func TestCollectLinks_TestFileUnknownAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	lines := []string{
		"// AUTH-99-1",
		"func TestLogin(t *testing.T) {}",
	}

	links := collectTestLinks(idx, lines)
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}

// LSP-4-5
func TestCollectLinks_TestFileNonAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	lines := []string{
		"func TestLogin(t *testing.T) {}",
	}

	links := collectTestLinks(idx, lines)
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}
