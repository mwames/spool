package lsp

import (
	"testing"

	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
	"go.lsp.dev/protocol"
)

// LSP-5-1
func TestBuildDiagnostics_UntestedAC(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		nil, // no test coverage
	)

	diags := buildDiagnostics(idx)

	// Should have one diagnostic for the .req file.
	reqURI := protocol.DocumentURI("file:///reqs/AUTH.req")
	fileDiags := diags[reqURI]
	if len(fileDiags) != 1 {
		t.Fatalf("len(diags) = %d, want 1", len(fileDiags))
	}
	if fileDiags[0].Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("severity = %v, want Warning", fileDiags[0].Severity)
	}
	if fileDiags[0].Range.Start.Line != 9 { // line 10 → 0-based 9
		t.Errorf("line = %d, want 9", fileDiags[0].Range.Start.Line)
	}
	if fileDiags[0].Source != "spool" {
		t.Errorf("source = %q, want spool", fileDiags[0].Source)
	}
}

// LSP-5-3
func TestBuildDiagnostics_TestedAC(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 5, "AUTH-1-1"))},
	)

	diags := buildDiagnostics(idx)
	if len(diags) != 0 {
		t.Errorf("len(diags) = %d, want 0 (AC is tested)", len(diags))
	}
}

// LSP-5-2
func TestBuildDiagnostics_OrphanedAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 5, "AUTH-99-1"))},
	)

	diags := buildDiagnostics(idx)

	testURI := protocol.DocumentURI("file:///tests/auth_test.go")
	fileDiags := diags[testURI]
	if len(fileDiags) != 1 {
		t.Fatalf("len(diags) = %d, want 1", len(fileDiags))
	}
	if fileDiags[0].Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("severity = %v, want Warning", fileDiags[0].Severity)
	}
	if fileDiags[0].Range.Start.Line != 3 { // annotation line 4 → 0-based 3
		t.Errorf("line = %d, want 3", fileDiags[0].Range.Start.Line)
	}
}

// LSP-5-4
func TestBuildDiagnostics_NoDiagnosticsWhenClean(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 10)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 5, "AUTH-1-1"))},
	)

	diags := buildDiagnostics(idx)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for clean index, got %d files", len(diags))
	}
}
