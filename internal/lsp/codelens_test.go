package lsp

import (
	"testing"

	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

// LSP-6-1
func TestCollectCodeLenses_ReqFileTestedAC(t *testing.T) {
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

	lenses := collectReqCodeLenses(idx, lines)

	var found bool
	for _, l := range lenses {
		if l.Range.Start.Line == 5 && l.Command != nil {
			found = true
			if l.Command.Title != "1 test(s)" {
				t.Errorf("title = %q, want %q", l.Command.Title, "1 test(s)")
			}
			if l.Command.Command != "spool.goToTests" {
				t.Errorf("command = %q, want spool.goToTests", l.Command.Command)
			}
		}
	}
	if !found {
		t.Fatal("no code lens found for AC on line 5")
	}
}

// LSP-6-2
func TestCollectCodeLenses_ReqFileUntestedAC(t *testing.T) {
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

	lenses := collectReqCodeLenses(idx, lines)
	var found bool
	for _, l := range lenses {
		if l.Range.Start.Line == 5 && l.Command != nil && l.Command.Title == "untested" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'untested' lens for AC on line 5")
	}
}

// LSP-6-3
func TestCollectCodeLenses_TestFileKnownAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 2, "AUTH-1-1"))},
	)

	lines := []string{
		"// AUTH-1-1",
		"func TestLogin(t *testing.T) {}",
	}

	lenses := collectTestCodeLenses(idx, "/tests/auth_test.go", lines)
	if len(lenses) != 1 {
		t.Fatalf("len(lenses) = %d, want 1", len(lenses))
	}
	if lenses[0].Command == nil {
		t.Fatal("expected command on test annotation lens")
	}
	if lenses[0].Command.Command != "spool.goToAC" {
		t.Errorf("command = %q, want spool.goToAC", lenses[0].Command.Command)
	}
}

// LSP-6-4
func TestCollectCodeLenses_TestFileUnknownAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil,
	)

	lines := []string{
		"// AUTH-99-1",
		"func TestLogin(t *testing.T) {}",
	}

	lenses := collectTestCodeLenses(idx, "/tests/auth_test.go", lines)
	if len(lenses) != 0 {
		t.Errorf("len(lenses) = %d, want 0", len(lenses))
	}
}
