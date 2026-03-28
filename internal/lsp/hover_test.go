package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

// LSP-7-1
func TestBuildHover_TestAnnotation(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "auth_test.go")
	os.WriteFile(testFile, []byte("// AUTH-1-1\nfunc TestLogin(t *testing.T) {}\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{{
			Feature:    "AUTH",
			SourceFile: "/reqs/AUTH.req",
			Requirements: []parser.Requirement{{
				ID:     "AUTH-1",
				Title:  "User Authentication",
				Status: "active",
				Line:   3,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "AUTH-1-1", Title: "Valid Credentials", Description: "Given valid creds", Line: 6},
				},
			}},
		}},
		[]scanner.FileResult{scanResult(testFile, mapping("TestLogin", 2, "AUTH-1-1"))},
	)

	hover := buildHover(idx, testFile, 0) // line 0 = "// AUTH-1-1"
	if hover == nil {
		t.Fatal("expected hover, got nil")
	}
	if !strings.Contains(hover.Contents.Value, "AUTH-1-1: Valid Credentials") {
		t.Errorf("hover missing AC title, got: %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "AUTH-1 — User Authentication") {
		t.Errorf("hover missing requirement info, got: %s", hover.Contents.Value)
	}
}

// LSP-7-2
func TestBuildHover_TestAnnotationUnknown(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "auth_test.go")
	os.WriteFile(testFile, []byte("// AUTH-99-1\nfunc TestLogin(t *testing.T) {}\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	hover := buildHover(idx, testFile, 0)
	if hover != nil {
		t.Errorf("expected nil hover for unknown annotation, got: %s", hover.Contents.Value)
	}
}

// LSP-7-3
func TestBuildHover_TestNonAnnotation(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "auth_test.go")
	os.WriteFile(testFile, []byte("func TestLogin(t *testing.T) {}\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	hover := buildHover(idx, testFile, 0)
	if hover != nil {
		t.Errorf("expected nil hover for non-annotation line")
	}
}

// LSP-7-4
func TestBuildHover_ReqFileACWithTests(t *testing.T) {
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
					{ID: "AUTH-1-1", Title: "Valid Credentials", Line: 6},
				},
			}},
		}},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 5, "AUTH-1-1"))},
	)

	hover := buildHover(idx, reqPath, 5) // "      - id: AUTH-1-1"
	if hover == nil {
		t.Fatal("expected hover, got nil")
	}
	if !strings.Contains(hover.Contents.Value, "Covered by 1 test(s)") {
		t.Errorf("hover missing test count, got: %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "TestLogin") {
		t.Errorf("hover missing test name, got: %s", hover.Contents.Value)
	}
}

// LSP-7-5
func TestBuildHover_ReqFileACUntested(t *testing.T) {
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
					{ID: "AUTH-1-1", Title: "Valid Credentials", Line: 6},
				},
			}},
		}},
		nil,
	)

	hover := buildHover(idx, reqPath, 5)
	if hover == nil {
		t.Fatal("expected hover, got nil")
	}
	if !strings.Contains(hover.Contents.Value, "No covering tests") {
		t.Errorf("hover missing untested message, got: %s", hover.Contents.Value)
	}
}

// LSP-7-6
func TestBuildHover_ReqFileNonIDLine(t *testing.T) {
	dir := t.TempDir()
	reqPath := filepath.Join(dir, "auth.req")
	os.WriteFile(reqPath, []byte("feature: AUTH\nrequirements:\n  - id: AUTH-1\n    title: Login\n"), 0644)

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	hover := buildHover(idx, reqPath, 0) // "feature: AUTH"
	if hover != nil {
		t.Errorf("expected nil hover for non-ID line")
	}
}
