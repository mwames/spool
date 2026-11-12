package lsp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func inv(id string, line int) parser.Invariant {
	return parser.Invariant{ID: id, Title: id + " title", Description: id + " description", Line: line}
}

// withInvariants returns r with the given invariants listed after its ACs,
// as parsed: one with no status of its own takes the requirement's
// (SPEC-4-1).
func withInvariants(r parser.Requirement, invs ...parser.Invariant) parser.Requirement {
	for i := range invs {
		if invs[i].Status == "" {
			invs[i].Status = r.Status
		}
	}
	r.Invariants = invs
	return r
}

// invReqContent is a .req file whose lines match invReqFile: AUTH-1 on line 3,
// AUTH-1-1 on line 7, AUTH-1-INV-1 on line 11 and AUTH-1-INV-2 on line 14.
const invReqContent = "feature: AUTH\n" +
	"requirements:\n" +
	"  - id: AUTH-1\n" +
	"    title: User Authentication\n" +
	"    status: active\n" +
	"    acceptance_criteria:\n" +
	"      - id: AUTH-1-1\n" +
	"        title: Valid Credentials\n" +
	"        description: Given valid creds\n" +
	"    invariants:\n" +
	"      - id: AUTH-1-INV-1\n" +
	"        title: No Plaintext Passwords\n" +
	"        description: Passwords are never logged\n" +
	"      - id: AUTH-1-INV-2\n" +
	"        title: Sessions Expire\n" +
	"        description: Every session has an expiry\n"

func invReqLines() []string {
	return strings.Split(invReqContent, "\n")
}

// invReqFile is the parsed form of invReqContent, stored at path.
func invReqFile(path string) *parser.ReqFile {
	return &parser.ReqFile{
		Feature:    "AUTH",
		SourceFile: path,
		Requirements: []parser.Requirement{{
			ID:     "AUTH-1",
			Title:  "User Authentication",
			Status: "active",
			Line:   3,
			AcceptanceCriteria: []parser.AcceptanceCriterion{
				{ID: "AUTH-1-1", Title: "Valid Credentials", Description: "Given valid creds", Status: "active", Line: 7},
			},
			Invariants: []parser.Invariant{
				{ID: "AUTH-1-INV-1", Title: "No Plaintext Passwords", Description: "Passwords are never logged", Status: "active", Line: 11},
				{ID: "AUTH-1-INV-2", Title: "Sessions Expire", Description: "Every session has an expiry", Status: "active", Line: 14},
			},
		}},
	}
}

func writeInvReq(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.req")
	writeFile(t, path, invReqContent)
	return path
}

func lineOf(t *testing.T, lines []string, text string) int {
	t.Helper()
	for i, l := range lines {
		if strings.Contains(l, text) {
			return i
		}
	}
	t.Fatalf("no line contains %q", text)
	return -1
}

func lineHasLens(lenses []protocol.CodeLens, line int) *protocol.CodeLens {
	for i := range lenses {
		if lenses[i].Range.Start.Line == uint32(line) && lenses[i].Command != nil {
			return &lenses[i]
		}
	}
	return nil
}

func linkOnLine(links []protocol.DocumentLink, line int) *protocol.DocumentLink {
	for i := range links {
		if links[i].Range.Start.Line == uint32(line) {
			return &links[i]
		}
	}
	return nil
}

type locArg struct {
	URI  string `json:"uri"`
	Line int    `json:"line"`
}

func decodeLocArgs(t *testing.T, args []interface{}) []locArg {
	t.Helper()
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	var out []locArg
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode command arguments %s: %v", b, err)
	}
	return out
}

// --- INVARIANT-2-1: go to definition ---

// INVARIANT-2-1
func TestResolveFromTest_AnnotationToInvariant(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "auth_test.go")
	writeFile(t, testFile, "// AUTH-1-INV-1: No Plaintext Passwords\nfunc TestNoPlaintext(t *testing.T) {}\n")

	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", withInvariants(req("AUTH-1", "active", ac("AUTH-1-1", 6)), inv("AUTH-1-INV-1", 12)))},
		[]scanner.FileResult{scanResult(testFile, mapping("TestNoPlaintext", 2, "AUTH-1-INV-1"))},
	)

	locs := resolve(idx, testFile, 0)
	if len(locs) != 1 {
		t.Fatalf("len(locs) = %d, want 1", len(locs))
	}
	if locs[0].URI != uri.File("/reqs/AUTH.req") {
		t.Errorf("URI = %q, want %q", locs[0].URI, uri.File("/reqs/AUTH.req"))
	}
	if locs[0].Range.Start.Line != 11 { // invariant line 12 → 0-based 11
		t.Errorf("Start.Line = %d, want 11", locs[0].Range.Start.Line)
	}
}

// INVARIANT-2-1
func TestResolveFromReq_InvariantToTests(t *testing.T) {
	reqPath := writeInvReq(t)
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile(reqPath)},
		[]scanner.FileResult{
			scanResult("/tests/a_test.go", mapping("TestNoPlaintextLog", 5, "AUTH-1-INV-1")),
			scanResult("/tests/b_test.go", mapping("TestNoPlaintextDump", 20, "AUTH-1-INV-1")),
		},
	)

	locs := resolve(idx, reqPath, lineOf(t, invReqLines(), "id: AUTH-1-INV-1"))
	if len(locs) != 2 {
		t.Fatalf("len(locs) = %d, want 2", len(locs))
	}
	want := map[protocol.DocumentURI]uint32{
		uri.File("/tests/a_test.go"): 4,
		uri.File("/tests/b_test.go"): 19,
	}
	for _, l := range locs {
		line, ok := want[l.URI]
		if !ok {
			t.Errorf("unexpected location %q", l.URI)
			continue
		}
		if l.Range.Start.Line != line {
			t.Errorf("%s: Start.Line = %d, want %d", l.URI, l.Range.Start.Line, line)
		}
		delete(want, l.URI)
	}
}

// INVARIANT-2-1
func TestResolveFromReq_UntestedInvariant(t *testing.T) {
	reqPath := writeInvReq(t)
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile(reqPath)},
		[]scanner.FileResult{scanResult("/tests/a_test.go", mapping("TestNoPlaintextLog", 5, "AUTH-1-INV-1"))},
	)

	locs := resolve(idx, reqPath, lineOf(t, invReqLines(), "id: AUTH-1-INV-2"))
	if len(locs) != 0 {
		t.Errorf("len(locs) = %d, want 0", len(locs))
	}
}

// --- INVARIANT-2-2: document links ---

// INVARIANT-2-2
func TestCollectLinks_ReqFileInvariants(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile("/reqs/AUTH.req")},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestNoPlaintext", 5, "AUTH-1-INV-1"))},
	)
	lines := invReqLines()

	links := collectReqLinks(idx, lines)

	tested := lineOf(t, lines, "id: AUTH-1-INV-1")
	link := linkOnLine(links, tested)
	if link == nil {
		t.Fatalf("no link on tested invariant line %d; links = %+v", tested, links)
	}
	idStart := uint32(strings.Index(lines[tested], "AUTH-1-INV-1"))
	if link.Range.Start.Character != idStart || link.Range.End.Character != idStart+uint32(len("AUTH-1-INV-1")) {
		t.Errorf("link range = %d-%d, want %d-%d", link.Range.Start.Character, link.Range.End.Character, idStart, idStart+uint32(len("AUTH-1-INV-1")))
	}
	if !strings.HasPrefix(string(link.Target), string(uri.File("/tests/auth_test.go"))) {
		t.Errorf("target = %q, want the covering test in %q", link.Target, uri.File("/tests/auth_test.go"))
	}

	untested := lineOf(t, lines, "id: AUTH-1-INV-2")
	if l := linkOnLine(links, untested); l != nil {
		t.Errorf("untested invariant has a link: %+v", *l)
	}
}

// INVARIANT-2-2
func TestCollectLinks_TestFileInvariantAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", withInvariants(req("AUTH-1", "active", ac("AUTH-1-1", 6)), inv("AUTH-1-INV-1", 12)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestNoPlaintext", 2, "AUTH-1-INV-1"))},
	)

	lines := []string{
		"// AUTH-1-INV-1",
		"func TestNoPlaintext(t *testing.T) {}",
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
	if links[0].Range.End.Character != 15 {
		t.Errorf("link end char = %d, want 15", links[0].Range.End.Character)
	}
	if !strings.HasPrefix(string(links[0].Target), string(uri.File("/reqs/AUTH.req"))) {
		t.Errorf("target = %q, want the invariant's .req file %q", links[0].Target, uri.File("/reqs/AUTH.req"))
	}
}

// --- INVARIANT-2-3: diagnostics ---

// INVARIANT-2-3
func TestBuildDiagnostics_UntestedInvariant(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", withInvariants(
			req("AUTH-1", "active", ac("AUTH-1-1", 6)),
			inv("AUTH-1-INV-1", 12),
			inv("AUTH-1-INV-2", 15),
		))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go",
			mapping("TestLogin", 5, "AUTH-1-1"),
			mapping("TestNoPlaintext", 10, "AUTH-1-INV-1"),
		)},
	)

	diags := buildDiagnostics(idx)

	fileDiags := diags[uri.File("/reqs/AUTH.req")]
	if len(fileDiags) != 1 {
		t.Fatalf("len(diags) = %d, want 1 (only the untested invariant): %+v", len(fileDiags), fileDiags)
	}
	d := fileDiags[0]
	if d.Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("severity = %v, want Warning", d.Severity)
	}
	if d.Range.Start.Line != 14 { // invariant line 15 → 0-based 14
		t.Errorf("line = %d, want 14", d.Range.Start.Line)
	}
	if !strings.Contains(d.Message, "AUTH-1-INV-2 title") || !strings.Contains(d.Message, "AUTH-1-INV-2") {
		t.Errorf("message = %q, want it to name the invariant's title and ID", d.Message)
	}
	if strings.Contains(d.Message, "AUTH-1-INV-1") {
		t.Errorf("message = %q names the tested invariant", d.Message)
	}
}

// INVARIANT-2-3
func TestBuildDiagnostics_OrphanedInvariantAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", withInvariants(req("AUTH-1", "active", ac("AUTH-1-1", 6)), inv("AUTH-1-INV-1", 12)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go",
			mapping("TestLogin", 5, "AUTH-1-1"),
			mapping("TestNoPlaintext", 10, "AUTH-1-INV-1"),
			mapping("TestGhost", 20, "AUTH-1-INV-9"),
		)},
	)

	diags := buildDiagnostics(idx)

	fileDiags := diags[uri.File("/tests/auth_test.go")]
	if len(fileDiags) != 1 {
		t.Fatalf("len(diags) = %d, want 1: %+v", len(fileDiags), fileDiags)
	}
	d := fileDiags[0]
	if d.Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("severity = %v, want Warning", d.Severity)
	}
	if d.Range.Start.Line != 18 { // annotation line 19 → 0-based 18
		t.Errorf("line = %d, want 18", d.Range.Start.Line)
	}
	if !strings.Contains(d.Message, "AUTH-1-INV-9") {
		t.Errorf("message = %q, want it to name AUTH-1-INV-9", d.Message)
	}
}

// --- INVARIANT-2-4: code lenses ---

// INVARIANT-2-4
func TestCollectCodeLenses_ReqFileInvariants(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile("/reqs/AUTH.req")},
		[]scanner.FileResult{
			scanResult("/tests/a_test.go", mapping("TestNoPlaintextLog", 5, "AUTH-1-INV-1")),
			scanResult("/tests/b_test.go", mapping("TestNoPlaintextDump", 20, "AUTH-1-INV-1")),
		},
	)
	lines := invReqLines()

	lenses := collectReqCodeLenses(idx, lines)

	tested := lineOf(t, lines, "id: AUTH-1-INV-1")
	l := lineHasLens(lenses, tested)
	if l == nil {
		t.Fatalf("no code lens on tested invariant line %d", tested)
	}
	if !strings.Contains(l.Command.Title, "2 test") {
		t.Errorf("title = %q, want the test count 2", l.Command.Title)
	}
	if l.Command.Command != "spool.goToTests" {
		t.Errorf("command = %q, want spool.goToTests", l.Command.Command)
	}
	args := decodeLocArgs(t, l.Command.Arguments)
	want := map[string]int{
		string(uri.File("/tests/a_test.go")): 4,
		string(uri.File("/tests/b_test.go")): 19,
	}
	if len(args) != len(want) {
		t.Fatalf("arguments = %+v, want one per covering test", args)
	}
	for _, a := range args {
		if line, ok := want[a.URI]; !ok || line != a.Line {
			t.Errorf("argument %+v does not locate a covering test %v", a, want)
		}
	}

	untested := lineOf(t, lines, "id: AUTH-1-INV-2")
	l = lineHasLens(lenses, untested)
	if l == nil {
		t.Fatalf("no code lens on untested invariant line %d", untested)
	}
	if l.Command.Title != "untested" {
		t.Errorf("title = %q, want %q", l.Command.Title, "untested")
	}
}

// INVARIANT-2-4
func TestCollectCodeLenses_TestFileInvariantAnnotation(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", withInvariants(req("AUTH-1", "active", ac("AUTH-1-1", 6)), inv("AUTH-1-INV-1", 12)))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestNoPlaintext", 2, "AUTH-1-INV-1"))},
	)

	lines := []string{
		"// AUTH-1-INV-1",
		"func TestNoPlaintext(t *testing.T) {}",
	}

	lenses := collectTestCodeLenses(idx, "/tests/auth_test.go", lines)
	if len(lenses) != 1 {
		t.Fatalf("len(lenses) = %d, want 1", len(lenses))
	}
	l := lenses[0]
	if l.Range.Start.Line != 0 {
		t.Errorf("lens line = %d, want 0", l.Range.Start.Line)
	}
	if l.Command == nil {
		t.Fatal("expected command on test annotation lens")
	}
	if !strings.Contains(l.Command.Title, "AUTH-1-INV-1 title") {
		t.Errorf("title = %q, want the invariant's title", l.Command.Title)
	}
	if l.Command.Command != "spool.goToAC" {
		t.Errorf("command = %q, want spool.goToAC", l.Command.Command)
	}
	args := decodeLocArgs(t, l.Command.Arguments)
	if len(args) != 1 || args[0].URI != string(uri.File("/reqs/AUTH.req")) || args[0].Line != 11 {
		t.Errorf("arguments = %+v, want [{%s 11}]", args, uri.File("/reqs/AUTH.req"))
	}
}

// --- INVARIANT-2-5: hover ---

// INVARIANT-2-5
func TestBuildHover_TestAnnotationInvariant(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "auth_test.go")
	writeFile(t, testFile, "// AUTH-1-INV-1\nfunc TestNoPlaintext(t *testing.T) {}\n")

	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile("/reqs/AUTH.req")},
		[]scanner.FileResult{scanResult(testFile, mapping("TestNoPlaintext", 2, "AUTH-1-INV-1"))},
	)

	hover := buildHover(idx, testFile, 0)
	if hover == nil {
		t.Fatal("expected hover, got nil")
	}
	for _, want := range []string{"AUTH-1-INV-1", "No Plaintext Passwords", "Passwords are never logged", "AUTH-1", "User Authentication"} {
		if !strings.Contains(hover.Contents.Value, want) {
			t.Errorf("hover missing %q, got: %s", want, hover.Contents.Value)
		}
	}
}

// INVARIANT-2-5
func TestBuildHover_ReqFileInvariantWithTests(t *testing.T) {
	reqPath := writeInvReq(t)
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile(reqPath)},
		[]scanner.FileResult{
			scanResult("/tests/a_test.go", mapping("TestNoPlaintextLog", 5, "AUTH-1-INV-1")),
			scanResult("/tests/b_test.go", mapping("TestNoPlaintextDump", 20, "AUTH-1-INV-1")),
		},
	)

	hover := buildHover(idx, reqPath, lineOf(t, invReqLines(), "id: AUTH-1-INV-1"))
	if hover == nil {
		t.Fatal("expected hover, got nil")
	}
	for _, want := range []string{"TestNoPlaintextLog", "a_test.go", "TestNoPlaintextDump", "b_test.go"} {
		if !strings.Contains(hover.Contents.Value, want) {
			t.Errorf("hover missing %q, got: %s", want, hover.Contents.Value)
		}
	}
	if strings.Contains(hover.Contents.Value, "No covering tests") {
		t.Errorf("tested invariant hover says no covering tests: %s", hover.Contents.Value)
	}
}

// INVARIANT-2-5
func TestBuildHover_ReqFileInvariantUntested(t *testing.T) {
	reqPath := writeInvReq(t)
	idx := buildTestIndex([]*parser.ReqFile{invReqFile(reqPath)}, nil)

	hover := buildHover(idx, reqPath, lineOf(t, invReqLines(), "id: AUTH-1-INV-2"))
	if hover == nil {
		t.Fatal("expected hover, got nil")
	}
	if !strings.Contains(hover.Contents.Value, "No covering tests") {
		t.Errorf("hover missing untested message, got: %s", hover.Contents.Value)
	}
}

// --- INVARIANT-2-6: requirement summaries ---

// INVARIANT-2-6
func TestBuildHover_RequirementListsInvariantsAfterACs(t *testing.T) {
	reqPath := writeInvReq(t)
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile(reqPath)},
		[]scanner.FileResult{
			scanResult("/tests/a_test.go", mapping("TestLogin", 3, "AUTH-1-1"), mapping("TestNoPlaintextLog", 5, "AUTH-1-INV-1")),
			scanResult("/tests/b_test.go", mapping("TestNoPlaintextDump", 20, "AUTH-1-INV-1")),
		},
	)

	hover := buildHover(idx, reqPath, 2) // "  - id: AUTH-1"
	if hover == nil {
		t.Fatal("expected hover, got nil")
	}
	md := hover.Contents.Value
	mdLines := strings.Split(md, "\n")

	acLine := -1
	inv1Line, inv2Line := -1, -1
	for i, l := range mdLines {
		switch {
		case strings.Contains(l, "AUTH-1-INV-1"):
			inv1Line = i
		case strings.Contains(l, "AUTH-1-INV-2"):
			inv2Line = i
		case strings.Contains(l, "AUTH-1-1"):
			acLine = i
		}
	}
	if acLine < 0 || inv1Line < 0 || inv2Line < 0 {
		t.Fatalf("hover does not list the AC and both invariants, got: %s", md)
	}
	if inv1Line < acLine || inv2Line < acLine {
		t.Errorf("invariants are not listed after the ACs, got: %s", md)
	}
	if !strings.Contains(mdLines[inv1Line], "2 test") {
		t.Errorf("tested invariant line = %q, want its test count 2", mdLines[inv1Line])
	}
	if !strings.Contains(mdLines[inv2Line], "untested") {
		t.Errorf("untested invariant line = %q, want %q", mdLines[inv2Line], "untested")
	}
}

// INVARIANT-2-6
func TestResolveFromReq_RequirementIncludesInvariantTestsAfterACs(t *testing.T) {
	reqPath := writeInvReq(t)
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile(reqPath)},
		[]scanner.FileResult{
			scanResult("/tests/inv_test.go", mapping("TestNoPlaintext", 30, "AUTH-1-INV-1")),
			scanResult("/tests/ac_test.go", mapping("TestLogin", 5, "AUTH-1-1")),
		},
	)

	locs := resolve(idx, reqPath, 2) // "  - id: AUTH-1"
	if len(locs) != 2 {
		t.Fatalf("len(locs) = %d, want 2 (AC test, then invariant test)", len(locs))
	}
	if locs[0].URI != uri.File("/tests/ac_test.go") || locs[0].Range.Start.Line != 4 {
		t.Errorf("locs[0] = %s:%d, want the AC's test %s:4", locs[0].URI, locs[0].Range.Start.Line, uri.File("/tests/ac_test.go"))
	}
	if locs[1].URI != uri.File("/tests/inv_test.go") || locs[1].Range.Start.Line != 29 {
		t.Errorf("locs[1] = %s:%d, want the invariant's test %s:29", locs[1].URI, locs[1].Range.Start.Line, uri.File("/tests/inv_test.go"))
	}
}

// INVARIANT-2-6
func TestCollectLinks_RequirementTargetsInvariantTestWhenNoACTest(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile("/reqs/AUTH.req")},
		[]scanner.FileResult{scanResult("/tests/inv_test.go", mapping("TestNoPlaintext", 30, "AUTH-1-INV-1"))},
	)
	lines := invReqLines()

	links := collectReqLinks(idx, lines)

	link := linkOnLine(links, 2) // "  - id: AUTH-1"
	if link == nil {
		t.Fatalf("no link on requirement line; links = %+v", links)
	}
	want := string(uri.File("/tests/inv_test.go")) + "#" + fmt.Sprintf("%d", 30)
	if string(link.Target) != want {
		t.Errorf("target = %q, want %q", link.Target, want)
	}
}

// INVARIANT-2-6
func TestCollectLinks_RequirementStillTargetsFirstACTest(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile("/reqs/AUTH.req")},
		[]scanner.FileResult{
			scanResult("/tests/inv_test.go", mapping("TestNoPlaintext", 30, "AUTH-1-INV-1")),
			scanResult("/tests/ac_test.go", mapping("TestLogin", 5, "AUTH-1-1")),
		},
	)

	links := collectReqLinks(idx, invReqLines())

	link := linkOnLine(links, 2) // "  - id: AUTH-1"
	if link == nil {
		t.Fatalf("no link on requirement line; links = %+v", links)
	}
	want := string(uri.File("/tests/ac_test.go")) + "#" + fmt.Sprintf("%d", 5)
	if string(link.Target) != want {
		t.Errorf("target = %q, want %q", link.Target, want)
	}
}

// --- INVARIANT-2-7: traceability tree ---

// INVARIANT-2-7
// SPEC-4-7
func TestBuildTraceabilityTree_Invariants(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile("/reqs/AUTH.req")},
		[]scanner.FileResult{
			scanResult("/tests/a_test.go", mapping("TestLogin", 3, "AUTH-1-1"), mapping("TestNoPlaintextLog", 5, "AUTH-1-INV-1")),
			scanResult("/tests/b_test.go", mapping("TestNoPlaintextDump", 20, "AUTH-1-INV-1")),
		},
	)

	tree := buildTraceabilityTree(idx)
	if len(tree.Features) != 1 || len(tree.Features[0].Requirements) != 1 {
		t.Fatalf("tree = %+v, want one feature with one requirement", tree)
	}
	r := tree.Features[0].Requirements[0]

	wantInvs := []TraceabilityInvariant{
		{ID: "AUTH-1-INV-1", Title: "No Plaintext Passwords", Status: "active", File: "/reqs/AUTH.req", Line: 11, TestCount: 2},
		{ID: "AUTH-1-INV-2", Title: "Sessions Expire", Status: "active", File: "/reqs/AUTH.req", Line: 14, TestCount: 0},
	}
	if len(r.Invariants) != len(wantInvs) {
		t.Fatalf("invariants = %+v, want %+v", r.Invariants, wantInvs)
	}
	for i, want := range wantInvs {
		if r.Invariants[i] != want {
			t.Errorf("invariants[%d] = %+v, want %+v", i, r.Invariants[i], want)
		}
	}

	wantACs := []TraceabilityAC{
		{ID: "AUTH-1-1", Title: "Valid Credentials", Status: "active", File: "/reqs/AUTH.req", Line: 7, TestCount: 1},
	}
	if len(r.ACs) != 1 || r.ACs[0] != wantACs[0] {
		t.Errorf("acs = %+v, want %+v", r.ACs, wantACs)
	}
}

// traceabilityJSON handles a spool/traceability request on a server holding
// idx and returns the first requirement of the first feature as raw JSON.
func traceabilityJSON(t *testing.T, idx *index.Index) map[string]json.RawMessage {
	t.Helper()
	srv := NewServer("")
	srv.idx = idx
	msg, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(2), "spool/traceability", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := call(t, srv, msg)

	var tree struct {
		Features []struct {
			Requirements []map[string]json.RawMessage `json:"requirements"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if len(tree.Features) != 1 || len(tree.Features[0].Requirements) != 1 {
		t.Fatalf("tree = %s, want one feature with one requirement", raw)
	}
	return tree.Features[0].Requirements[0]
}

// INVARIANT-2-7
func TestHandleTraceability_InvariantsJSON(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{invReqFile("/reqs/AUTH.req")},
		[]scanner.FileResult{scanResult("/tests/a_test.go", mapping("TestNoPlaintextLog", 5, "AUTH-1-INV-1"))},
	)

	r := traceabilityJSON(t, idx)

	var invs []map[string]interface{}
	if err := json.Unmarshal(r["invariants"], &invs); err != nil {
		t.Fatalf("decode invariants %s: %v", r["invariants"], err)
	}
	want := []map[string]interface{}{
		{"id": "AUTH-1-INV-1", "title": "No Plaintext Passwords", "file": "/reqs/AUTH.req", "line": float64(11), "testCount": float64(1)},
		{"id": "AUTH-1-INV-2", "title": "Sessions Expire", "file": "/reqs/AUTH.req", "line": float64(14), "testCount": float64(0)},
	}
	if len(invs) != len(want) {
		t.Fatalf("invariants = %s, want %v", r["invariants"], want)
	}
	for i := range want {
		for k, v := range want[i] {
			if invs[i][k] != v {
				t.Errorf("invariants[%d][%q] = %v, want %v", i, k, invs[i][k], v)
			}
		}
	}

	var acs []map[string]interface{}
	if err := json.Unmarshal(r["acs"], &acs); err != nil {
		t.Fatalf("decode acs %s: %v", r["acs"], err)
	}
	if len(acs) != 1 || acs[0]["id"] != "AUTH-1-1" || acs[0]["testCount"] != float64(0) {
		t.Errorf("acs = %s, want only AUTH-1-1, untested", r["acs"])
	}
}

// INVARIANT-2-7
func TestHandleTraceability_NoInvariantsIsEmptyList(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active", ac("AUTH-1-1", 6)))},
		nil,
	)

	r := traceabilityJSON(t, idx)

	got, ok := r["invariants"]
	if !ok {
		t.Fatal(`requirement has no "invariants" field`)
	}
	if strings.TrimSpace(string(got)) != "[]" {
		t.Errorf(`"invariants" = %s, want []`, got)
	}
}

func TestInvariantFixture_MatchesContent(t *testing.T) {
	// Guards the fixture: invReqFile's line numbers must match invReqContent.
	lines := invReqLines()
	rf := invReqFile("")
	r := rf.Requirements[0]
	check := func(id string, line int) {
		if !strings.Contains(lines[line-1], "id: "+id) {
			t.Errorf("line %d = %q, want id %s", line, lines[line-1], id)
		}
	}
	check(r.ID, r.Line)
	for _, a := range r.AcceptanceCriteria {
		check(a.ID, a.Line)
	}
	for _, v := range r.Invariants {
		check(v.ID, v.Line)
	}
}
