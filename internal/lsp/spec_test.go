package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// SPEC-1-5
func TestInterfaceDiagnostics_MissingPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "stub"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "stub", "present.go"), "package stub\n")

	one := req("AUTH-1", "active", ac("AUTH-1-1", 6))
	one.Line = 3
	one.Interface = parser.StringList{"stub/present.go", "stub/missing.go", "stub"}
	two := req("AUTH-2", "active", ac("AUTH-2-1", 14))
	two.Line = 11
	two.Interface = parser.StringList{"./gone/a.go", "gone/dir"}
	clean := req("AUTH-3", "active", ac("AUTH-3-1", 22))
	clean.Line = 19
	clean.Interface = parser.StringList{"stub/present.go"}
	draft := req("AUTH-4", "draft", ac("AUTH-4-1", 30))
	draft.Line = 27
	draft.Interface = parser.StringList{"draft/missing.go"}
	deprecated := req("AUTH-5", "deprecated", ac("AUTH-5-1", 38))
	deprecated.Line = 35
	deprecated.Interface = parser.StringList{"deprecated/missing.go"}
	superseded := req("AUTH-6", "superseded", parser.AcceptanceCriterion{ID: "AUTH-6-1", Title: "AUTH-6-1", Description: "Given a, when b, then c", Status: "active", Line: 46})
	superseded.Line = 43
	superseded.Interface = parser.StringList{"superseded/missing.go"}

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", one, two, clean, draft, deprecated, superseded)},
		[]scanner.FileResult{scanResult("/tests/auth_test.go",
			mapping("TestOne", 5, "AUTH-1-1"),
			mapping("TestTwo", 8, "AUTH-2-1"),
			mapping("TestThree", 11, "AUTH-3-1"),
		)},
	)

	diags := interfaceDiagnostics(idx, root)

	fileDiags := diags[uri.File("/reqs/AUTH.req")]
	wantLine := map[string]uint32{ // missing path → 0-based ID line
		"stub/missing.go":  2,
		"./gone/a.go":      10,
		"gone/dir":         10,
		"draft/missing.go": 26,
	}
	for path, line := range wantLine {
		found := 0
		for _, d := range fileDiags {
			if d.Range.Start.Line == line && strings.Contains(d.Message, path) {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%d diagnostics on line %d naming %q, want 1, in %+v", found, line, path, fileDiags)
		}
	}
	if len(fileDiags) != len(wantLine) {
		t.Errorf("len(diags) = %d, want %d (one per missing path): %+v", len(fileDiags), len(wantLine), fileDiags)
	}
	for _, d := range fileDiags {
		if d.Severity != protocol.DiagnosticSeverityWarning {
			t.Errorf("diagnostic %q: severity = %v, want Warning", d.Message, d.Severity)
		}
		if d.Range.Start.Line != 2 && d.Range.Start.Line != 10 && d.Range.Start.Line != 26 {
			t.Errorf("diagnostic %q on line %d, want only the ID lines of requirements with a missing path", d.Message, d.Range.Start.Line)
		}
		if strings.Contains(d.Message, "present.go") {
			t.Errorf("diagnostic %q names a path that exists", d.Message)
		}
		if strings.Contains(d.Message, "deprecated/missing.go") || strings.Contains(d.Message, "superseded/missing.go") {
			t.Errorf("diagnostic %q names a path of a deprecated or superseded requirement", d.Message)
		}
	}
	for fileURI, ds := range diags {
		if fileURI != uri.File("/reqs/AUTH.req") && len(ds) > 0 {
			t.Errorf("diagnostics for %s, want only the .req file: %+v", fileURI, ds)
		}
	}
}

// SPEC-3-4
func TestBuildDiagnostics_NotGivenWhenThen(t *testing.T) {
	unformed := parser.AcceptanceCriterion{ID: "AUTH-1-2", Title: "Unformed", Description: "Given valid creds", Line: 9}
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 6), unformed))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go",
			mapping("TestOne", 5, "AUTH-1-1"),
			mapping("TestTwo", 8, "AUTH-1-2"),
		)},
	)

	diags := buildDiagnostics(idx)

	fileDiags := diags[uri.File("/reqs/AUTH.req")]
	if len(fileDiags) != 1 {
		t.Fatalf("len(diags) = %d, want 1 (only the AC not written as Given/When/Then): %+v", len(fileDiags), fileDiags)
	}
	d := fileDiags[0]
	if d.Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("severity = %v, want Warning", d.Severity)
	}
	if d.Range.Start.Line != 8 { // AC line 9 → 0-based 8
		t.Errorf("line = %d, want 8", d.Range.Start.Line)
	}
	if !strings.Contains(d.Message, "Given/When/Then") {
		t.Errorf("message = %q, want it to say the AC isn't written as Given/When/Then", d.Message)
	}
}

// SPEC-3-4
func TestBuildDiagnostics_GivenWhenThenNoDiagnostic(t *testing.T) {
	formed := parser.AcceptanceCriterion{ID: "AUTH-1-2", Title: "Formed", Description: "GIVEN valid creds\nWHEN they log in\nTHEN it succeeds\n", Line: 9}
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 6), formed))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go",
			mapping("TestOne", 5, "AUTH-1-1"),
			mapping("TestTwo", 8, "AUTH-1-2"),
		)},
	)

	if diags := buildDiagnostics(idx); len(diags[uri.File("/reqs/AUTH.req")]) != 0 {
		t.Errorf("diagnostics = %+v, want none for ACs written as Given/When/Then", diags)
	}
}

// SPEC-3-4
func TestBuildDiagnostics_GivenWhenThenNotActiveOrInvariant(t *testing.T) {
	unformed := func(id, status string, line int) parser.AcceptanceCriterion {
		return parser.AcceptanceCriterion{ID: id, Title: "Unformed", Description: "Spool reports it.", Status: status, Line: line}
	}
	active := req("AUTH-1", "active", ac("AUTH-1-1", 6), unformed("AUTH-1-2", "deprecated", 8))
	active.Line = 3
	active.Invariants = []parser.Invariant{
		{ID: "AUTH-1-INV-1", Title: "Holds", Description: "Passwords are never logged", Status: "active", Line: 9},
	}
	deprecated := req("AUTH-2", "deprecated", unformed("AUTH-2-1", "", 15))
	deprecated.Line = 12
	superseded := req("AUTH-3", "superseded", unformed("AUTH-3-1", "active", 21), unformed("AUTH-3-2", "draft", 23))
	superseded.Line = 18
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", active, deprecated, superseded)},
		[]scanner.FileResult{scanResult("/tests/auth_test.go",
			mapping("TestOne", 5, "AUTH-1-1"),
			mapping("TestInv", 8, "AUTH-1-INV-1"),
		)},
	)

	if diags := buildDiagnostics(idx); len(diags[uri.File("/reqs/AUTH.req")]) != 0 {
		t.Errorf("diagnostics = %+v, want none for invariants, deprecated ACs or ACs of superseded requirements", diags)
	}
}

// SPEC-3-4
func TestBuildDiagnostics_GivenWhenThenDraft(t *testing.T) {
	unformed := func(id, status string, line int) parser.AcceptanceCriterion {
		return parser.AcceptanceCriterion{ID: id, Title: "Unformed " + id, Description: "Spool reports it.", Status: status, Line: line}
	}
	// AUTH-1 is written active and folded draft by its draft AC; AUTH-2 is
	// written draft.
	withDraft := req("AUTH-1", "draft", ac("AUTH-1-1", 6), unformed("AUTH-1-2", "draft", 9))
	withDraft.Line = 3
	draft := req("AUTH-2", "draft", unformed("AUTH-2-1", "", 15))
	draft.Line = 12
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", withDraft, draft)},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestOne", 5, "AUTH-1-1"))},
	)

	fileDiags := buildDiagnostics(idx)[uri.File("/reqs/AUTH.req")]
	want := map[uint32]string{8: "AUTH-1-2", 14: "AUTH-2-1"} // 0-based AC lines
	if len(fileDiags) != len(want) {
		t.Fatalf("len(diags) = %d, want %d (one per draft AC not written as Given/When/Then): %+v", len(fileDiags), len(want), fileDiags)
	}
	for _, d := range fileDiags {
		id, ok := want[d.Range.Start.Line]
		if !ok {
			t.Errorf("diagnostic %q on line %d, want only lines %v", d.Message, d.Range.Start.Line, want)
			continue
		}
		if !strings.Contains(d.Message, "Given/When/Then") {
			t.Errorf("diagnostic %q on %s's line, want it to say it isn't written as Given/When/Then", d.Message, id)
		}
		if d.Severity != protocol.DiagnosticSeverityWarning {
			t.Errorf("diagnostic %q: severity = %v, want Warning", d.Message, d.Severity)
		}
	}
}

// scopeReqContent is a .req file with AUTH-1 on line 3, AUTH-1-1 on line 6
// and AUTH-1-INV-1 on line 10.
const scopeReqContent = "feature: AUTH\n" +
	"requirements:\n" +
	"  - id: AUTH-1\n" +
	"    title: User Authentication\n" +
	"    acceptance_criteria:\n" +
	"      - id: AUTH-1-1\n" +
	"        title: Valid Credentials\n" +
	"        description: Given valid creds, when they log in, then it succeeds\n" +
	"    invariants:\n" +
	"      - id: AUTH-1-INV-1\n" +
	"        title: No Plaintext Passwords\n" +
	"        description: Passwords are never logged\n"

// scopeHover returns the hover for the ID line of AUTH-1, whose AC is tested
// and whose invariant isn't, with the given out_of_scope items.
func scopeHover(t *testing.T, outOfScope parser.StringList) string {
	t.Helper()
	reqPath := filepath.Join(t.TempDir(), "auth.req")
	writeFile(t, reqPath, scopeReqContent)
	idx := buildTestIndex(
		[]*parser.ReqFile{{
			Feature:    "AUTH",
			SourceFile: reqPath,
			Requirements: []parser.Requirement{{
				ID:         "AUTH-1",
				Title:      "User Authentication",
				Status:     "active",
				Line:       3,
				OutOfScope: outOfScope,
				AcceptanceCriteria: []parser.AcceptanceCriterion{
					{ID: "AUTH-1-1", Title: "Valid Credentials", Description: "Given valid creds, when they log in, then it succeeds", Line: 6},
				},
				Invariants: []parser.Invariant{
					{ID: "AUTH-1-INV-1", Title: "No Plaintext Passwords", Description: "Passwords are never logged", Line: 10},
				},
			}},
		}},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 5, "AUTH-1-1"))},
	)
	hover := buildHover(idx, reqPath, 2) // "  - id: AUTH-1"
	if hover == nil {
		t.Fatal("expected hover, got nil")
	}
	return hover.Contents.Value
}

// scopeHoverToday is the requirement hover for scopeReqContent before
// out_of_scope was shown.
const scopeHoverToday = "**AUTH-1: User Authentication**\n\n" +
	"- AUTH-1-1: 1 test(s)\n" +
	"- AUTH-1-INV-1: untested\n"

// SPEC-2-3
func TestBuildHover_RequirementOutOfScope(t *testing.T) {
	items := []string{"Globs in interface paths", "Checking what the stub contains", "Linting what it says"}
	md := scopeHover(t, parser.StringList(items))

	if !strings.HasPrefix(md, scopeHoverToday) {
		t.Fatalf("hover =\n%s\nwant it to start with what it shows today\n%s", md, scopeHoverToday)
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimPrefix(md, scopeHoverToday), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, strings.TrimSpace(l))
		}
	}
	if len(lines) != 1+len(items) {
		t.Fatalf("hover after today's content = %q, want an \"Out of scope\" line and %d bullets\nhover:\n%s", lines, len(items), md)
	}
	if !strings.Contains(lines[0], "Out of scope") {
		t.Errorf("line %q, want an \"Out of scope\" line\nhover:\n%s", lines[0], md)
	}
	for i, item := range items {
		l := lines[1+i]
		if !strings.HasPrefix(l, "- ") && !strings.HasPrefix(l, "* ") {
			t.Errorf("line %q is not a bullet\nhover:\n%s", l, md)
			continue
		}
		if got := strings.TrimSpace(l[2:]); got != item {
			t.Errorf("bullet %d = %q, want %q\nhover:\n%s", i+1, got, item, md)
		}
	}
}

// SPEC-2-3
func TestBuildHover_RequirementNoOutOfScope(t *testing.T) {
	for name, items := range map[string]parser.StringList{
		"none":  nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			if md := scopeHover(t, items); md != scopeHoverToday {
				t.Errorf("hover =\n%s\nwant it unchanged\n%s", md, scopeHoverToday)
			}
		})
	}
}
