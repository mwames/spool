package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/project"
	"github.com/mwames/Spool/internal/reporter"
	"github.com/mwames/Spool/internal/scanner"
	"github.com/mwames/Spool/internal/spoolid"
)

// demoReq defines DEMO-1-1, DEMO-1-2 and DEMO-1-INV-1, which demoTest links
// to tests, and DEMO-1-3 and DEMO-1-INV-2, which it leaves untested.
const demoReq = `feature: DEMO

requirements:

  - id: DEMO-1
    title: Demo
    description: A demo requirement.
    status: active
    acceptance_criteria:
      - id: DEMO-1-1
        title: One
        description: The first AC.
      - id: DEMO-1-2
        title: Two
        description: The second AC.
      - id: DEMO-1-3
        title: Three
        description: The third AC, untested.
    invariants:
      - id: DEMO-1-INV-1
        title: Invariant One
        description: The first invariant.
      - id: DEMO-1-INV-2
        title: Invariant Two
        description: The second invariant, untested.
`

// demoTestPath is where demoTest is written, relative to the project root.
const demoTestPath = "pkg/demo_test.go"

// demoTest links DEMO-1-1 to TestOne (line 6) and TestBoth (line 13),
// DEMO-1-2 to TestTwo (line 9), DEMO-1-INV-1 to TestBoth, and has an orphaned
// annotation for DEMO-9-9. It is built from lines so that the annotations
// aren't read as annotations of this file.
var demoTest = strings.Join([]string{
	"package pkg",
	"",
	`import "testing"`,
	"",
	"// DEMO-1-1",
	"func TestOne(t *testing.T) {}",
	"",
	"// DEMO-1-2",
	"func TestTwo(t *testing.T) {}",
	"",
	"// DEMO-1-1",
	"// DEMO-1-INV-1",
	"func TestBoth(t *testing.T) {}",
	"",
	"// DEMO-9-9",
	"func TestOrphan(t *testing.T) {}",
	"",
}, "\n")

// earlyTestPath sorts before demoTestPath and lateTestPath after it.
const (
	earlyTestPath = "a/b/early_test.go"
	lateTestPath  = "pkg/zz_test.go"
)

// earlyTest links DEMO-1-1 to TestEarly (line 20) and DEMO-1-INV-1 to
// TestEarlyInv (line 23), both later in their file than demoTest's tests.
var earlyTest = strings.Join(append(append([]string{
	"package b",
	"",
	`import "testing"`,
}, make([]string, 15)...),
	"// DEMO-1-1",
	"func TestEarly(t *testing.T) {}",
	"",
	"// DEMO-1-INV-1",
	"func TestEarlyInv(t *testing.T) {}",
	"",
), "\n")

// lateTest links DEMO-1-1 to TestLate (line 5), earlier in its file than
// demoTest's tests.
var lateTest = strings.Join([]string{
	"package pkg",
	"",
	`import "testing"`,
	"// DEMO-1-1",
	"func TestLate(t *testing.T) {}",
	"",
}, "\n")

// severityConfig returns a .spool.yaml that scans Go test files with the
// given severities for untested ACs, untested invariants and orphaned
// annotations.
func severityConfig(untestedAC, untestedInvariant, orphaned config.Severity) string {
	return "test_patterns:\n" +
		"  - \"**/*_test.go\"\n" +
		"severity:\n" +
		"  untested_ac: " + string(untestedAC) + "\n" +
		"  untested_invariant: " + string(untestedInvariant) + "\n" +
		"  orphaned_annotation: " + string(orphaned) + "\n"
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// demoProject writes the demo project with the given .spool.yaml into a
// temporary directory and returns its root.
func demoProject(t *testing.T, spoolYAML string) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".spool.yaml":     spoolYAML,
		".spool/demo.req": demoReq,
		demoTestPath:      demoTest,
	})
	return root
}

// multiFileProject is the demo project plus earlyTest and lateTest, so that
// DEMO-1-1 and DEMO-1-INV-1 have tests in several files whose order by line
// alone differs from their order by file, then line.
func multiFileProject(t *testing.T, spoolYAML string) string {
	t.Helper()
	root := demoProject(t, spoolYAML)
	writeFiles(t, root, map[string]string{
		earlyTestPath: earlyTest,
		lateTestPath:  lateTest,
	})
	return root
}

// noisyProject is the demo project plus a .req file that fails to parse, a
// requirement whose prefix doesn't match its feature, and a file matching
// test_patterns that no interpreter supports, so that a run produces parse,
// scan and index warnings and errors as well as a report.
func noisyProject(t *testing.T) string {
	t.Helper()
	root := demoProject(t, "test_patterns:\n  - \"**/*_test.go\"\n  - \"**/*.check\"\n")
	writeFiles(t, root, map[string]string{
		".spool/broken.req": "feature: [unclosed\n",
		".spool/other.req": `feature: OTHER

requirements:

  - id: DEMO-5
    title: Wrong Prefix
    description: A requirement whose prefix is not its feature.
    status: active
    acceptance_criteria:
      - id: DEMO-5-1
        title: Five
        description: An AC.
`,
		"pkg/notes.check": "nothing to see\n",
	})
	return root
}

// run calls Run in root and returns its exit status, stdout and stderr.
func run(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, root, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// wholeReport returns the whole-repo report for root in the given format and
// its exit status under the project's severities, from the pipeline spool ran
// before --format and --ids existed.
func wholeReport(t *testing.T, root string, f spool.Formatter) (string, int) {
	t.Helper()
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	pr, err := parser.ParseDir(cfg.ReqsDir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := project.FindTestFiles(root, cfg.TestPatterns, cfg.ExcludePatterns)
	if err != nil {
		t.Fatal(err)
	}
	sr := scanner.Scan(files, project.DefaultInterpreters())
	r := reporter.Generate(index.Build(pr.Files, sr.Files))
	data, err := f.Format(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), reporter.ExitCode(r, cfg.Severity)
}

// textEntry is one listed ID as read back from the --ids text format. Tests
// are the trimmed "<file>:<line> <function>" lines, in the order printed.
type textEntry struct {
	ID    string
	State string
	Tests []string
}

// parseCoverageText reads the --ids text format: an unindented line per ID
// holding the ID followed by its state, its tests on indented lines below,
// and a last line "<n>/<m> covered", which is returned trimmed. Any other
// line, blank ones included, fails the test.
func parseCoverageText(t *testing.T, out string) ([]textEntry, string) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	var entries []textEntry
	for _, l := range lines[:len(lines)-1] {
		if strings.TrimSpace(l) == "" {
			t.Fatalf("blank line before the last line in:\n%s", out)
		}
		if l[0] == ' ' || l[0] == '\t' {
			if len(entries) == 0 {
				t.Fatalf("indented line %q before any ID line in:\n%s", l, out)
			}
			e := &entries[len(entries)-1]
			e.Tests = append(e.Tests, strings.TrimSpace(l))
			continue
		}
		var e textEntry
		for _, tok := range strings.FieldsFunc(l, func(r rune) bool {
			return unicode.IsSpace(r) || strings.ContainsRune(":()[],", r)
		}) {
			switch {
			case tok == "covered" || tok == "untested" || tok == "unknown":
				if e.State != "" {
					t.Fatalf("ID line %q gives more than one state", l)
				}
				if e.ID == "" {
					t.Fatalf("ID line %q gives the state before the ID", l)
				}
				e.State = tok
			case e.ID == "" && spoolid.IsValid(tok):
				e.ID = tok
			}
		}
		if e.ID == "" || e.State == "" {
			t.Fatalf("ID line %q does not give an ID and a state (covered, untested or unknown) in:\n%s", l, out)
		}
		entries = append(entries, e)
	}
	return entries, last
}

// checkCoverageText asserts that out is the --ids text format with exactly
// the wanted entries and tests, in order, and the wanted last line.
func checkCoverageText(t *testing.T, out string, want []textEntry, wantLast string) {
	t.Helper()
	got, last := parseCoverageText(t, out)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %+v\nwant %+v\noutput:\n%s", got, want, out)
	}
	if last != wantLast {
		t.Errorf("last line = %q, want %q\noutput:\n%s", last, wantLast, out)
	}
}

// demoEntry returns the wanted text entry for a demo project ID.
func demoEntry(id string) textEntry {
	tests := map[string][]string{
		"DEMO-1-1":     {demoTestPath + ":6 TestOne", demoTestPath + ":13 TestBoth"},
		"DEMO-1-2":     {demoTestPath + ":9 TestTwo"},
		"DEMO-1-INV-1": {demoTestPath + ":13 TestBoth"},
	}
	switch id {
	case "DEMO-1-1", "DEMO-1-2", "DEMO-1-INV-1":
		return textEntry{ID: id, State: "covered", Tests: tests[id]}
	case "DEMO-1-3", "DEMO-1-INV-2":
		return textEntry{ID: id, State: "untested"}
	}
	return textEntry{ID: id, State: "unknown"}
}

func demoEntries(ids ...string) []textEntry {
	var out []textEntry
	for _, id := range ids {
		out = append(out, demoEntry(id))
	}
	return out
}

// checkUsageError asserts a usage error: exit 2, nothing on stdout, and a
// message on stderr containing each of the wanted strings.
func checkUsageError(t *testing.T, code int, stdout, stderr string, want ...string) {
	t.Helper()
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Errorf("stderr is empty, want a message")
	}
	for _, w := range want {
		if !strings.Contains(stderr, w) {
			t.Errorf("stderr = %q, want it to name %q", stderr, w)
		}
	}
}

// jsonCoverage mirrors the --ids JSON format.
type jsonCoverage struct {
	IDs []struct {
		ID      string          `json:"id"`
		Kind    string          `json:"kind"`
		Covered bool            `json:"covered"`
		Tests   json.RawMessage `json:"tests"`
	} `json:"ids"`
	Covered int `json:"covered"`
	Total   int `json:"total"`
}

type jsonCoverageTest struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// decodeOne decodes out as exactly one JSON value into v, with nothing but
// whitespace after it.
func decodeOne(t *testing.T, out string, v any) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(out))
	if err := dec.Decode(v); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("stdout holds more than one JSON value:\n%s", out)
	}
}

// keys returns the sorted keys of a JSON object.
func keys(t *testing.T, raw []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not a JSON object: %v\n%s", err, raw)
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// COVERAGE-1-1
func TestRun_TextByDefault(t *testing.T) {
	for _, tc := range []struct {
		name     string
		yaml     string
		wantExit int
	}{
		{"error severities", severityConfig(config.SeverityError, config.SeverityError, config.SeverityError), 1},
		{"lower severities", severityConfig(config.SeverityWarning, config.SeverityInfo, config.SeverityOff), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := demoProject(t, tc.yaml)
			want, wantExit := wholeReport(t, root, reporter.Text{})
			if wantExit != tc.wantExit {
				t.Fatalf("fixture: report exit = %d, want %d", wantExit, tc.wantExit)
			}
			code, stdout, _ := run(t, root)
			if stdout != want {
				t.Errorf("stdout =\n%s\nwant the text report\n%s", stdout, want)
			}
			if code != wantExit {
				t.Errorf("exit = %d, want %d", code, wantExit)
			}
		})
	}
}

// COVERAGE-1-2
func TestRun_FormatJSON(t *testing.T) {
	for _, tc := range []struct {
		name     string
		yaml     string
		wantExit int
	}{
		{"error severities", severityConfig(config.SeverityError, config.SeverityError, config.SeverityError), 1},
		{"orphaned error only", severityConfig(config.SeverityOff, config.SeverityOff, config.SeverityError), 1},
		{"lower severities", severityConfig(config.SeverityWarning, config.SeverityInfo, config.SeverityOff), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := demoProject(t, tc.yaml)
			want, wantExit := wholeReport(t, root, reporter.JSON{})
			if wantExit != tc.wantExit {
				t.Fatalf("fixture: report exit = %d, want %d", wantExit, tc.wantExit)
			}
			for _, args := range [][]string{{"--format", "json"}, {"--format=json"}} {
				code, stdout, _ := run(t, root, args...)
				if stdout != want {
					t.Errorf("%v: stdout =\n%s\nwant exactly the JSON report\n%s", args, stdout, want)
				}
				if code != wantExit {
					t.Errorf("%v: exit = %d, want %d", args, code, wantExit)
				}
			}
		})
	}
}

// COVERAGE-1-3
func TestRun_FormatMarkdown(t *testing.T) {
	for _, tc := range []struct {
		name     string
		yaml     string
		wantExit int
	}{
		{"error severities", severityConfig(config.SeverityError, config.SeverityError, config.SeverityError), 1},
		{"untested invariant error only", severityConfig(config.SeverityOff, config.SeverityError, config.SeverityOff), 1},
		{"lower severities", severityConfig(config.SeverityWarning, config.SeverityInfo, config.SeverityOff), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := demoProject(t, tc.yaml)
			want, wantExit := wholeReport(t, root, reporter.Markdown{})
			if wantExit != tc.wantExit {
				t.Fatalf("fixture: report exit = %d, want %d", wantExit, tc.wantExit)
			}
			for _, args := range [][]string{{"--format", "markdown"}, {"--format=markdown"}} {
				code, stdout, _ := run(t, root, args...)
				if stdout != want {
					t.Errorf("%v: stdout =\n%s\nwant exactly the markdown report\n%s", args, stdout, want)
				}
				if code != wantExit {
					t.Errorf("%v: exit = %d, want %d", args, code, wantExit)
				}
			}
		})
	}
}

// COVERAGE-1-4
func TestRun_UsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown format", []string{"--format", "xml"}, "xml"},
		{"unknown format with equals", []string{"--format=xml"}, "xml"},
		{"unknown format with ids", []string{"--ids", "DEMO-1-1", "--format=xml"}, "xml"},
		{"unknown flag", []string{"--verbose"}, "--verbose"},
		{"unknown flag with equals", []string{"--verbose=yes"}, "--verbose"},
		{"unknown flag with format", []string{"--format", "json", "--verbose"}, "--verbose"},
		{"unknown flag before format", []string{"--verbose", "--format=json"}, "--verbose"},
		{"unexpected argument", []string{"extra"}, "extra"},
		{"unexpected argument after format", []string{"--format", "text", "extra"}, "extra"},
		{"unexpected argument after format with equals", []string{"--format=text", "extra"}, "extra"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := demoProject(t, severityConfig(config.SeverityOff, config.SeverityOff, config.SeverityOff))
			code, stdout, stderr := run(t, root, tc.args...)
			checkUsageError(t, code, stdout, stderr, tc.want)
		})
	}
}

// COVERAGE-1-INV-1
func TestRun_StdoutHoldsOnlyTheReport(t *testing.T) {
	ids := "DEMO-1-1,DEMO-1-3"
	idsWant := demoEntries("DEMO-1-1", "DEMO-1-3")
	for _, tc := range []struct {
		name string
		args []string
		// whole is the formatter for a whole-repo report, nil otherwise.
		whole spool.Formatter
		// idsText and idsJSON mark an --ids report in that format.
		idsText bool
		idsJSON bool
	}{
		{name: "no flags", whole: reporter.Text{}},
		{name: "format text", args: []string{"--format", "text"}, whole: reporter.Text{}},
		{name: "format=text", args: []string{"--format=text"}, whole: reporter.Text{}},
		{name: "format json", args: []string{"--format", "json"}, whole: reporter.JSON{}},
		{name: "format=json", args: []string{"--format=json"}, whole: reporter.JSON{}},
		{name: "format markdown", args: []string{"--format", "markdown"}, whole: reporter.Markdown{}},
		{name: "format=markdown", args: []string{"--format=markdown"}, whole: reporter.Markdown{}},
		{name: "ids", args: []string{"--ids", ids}, idsText: true},
		{name: "ids=", args: []string{"--ids=" + ids}, idsText: true},
		{name: "ids format text", args: []string{"--ids", ids, "--format", "text"}, idsText: true},
		{name: "ids format json", args: []string{"--ids", ids, "--format", "json"}, idsJSON: true},
		{name: "format json ids", args: []string{"--format", "json", "--ids", ids}, idsJSON: true},
		{name: "format=json ids=", args: []string{"--format=json", "--ids=" + ids}, idsJSON: true},
		{name: "unknown format", args: []string{"--format", "xml"}},
		{name: "unknown flag", args: []string{"--verbose"}},
		{name: "unexpected argument", args: []string{"extra"}},
		{name: "requirement id", args: []string{"--ids", "DEMO-1"}},
		{name: "ids format markdown", args: []string{"--ids", ids, "--format", "markdown"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := noisyProject(t)
			_, stdout, stderr := run(t, root, tc.args...)
			if strings.TrimSpace(stderr) == "" {
				t.Errorf("stderr is empty, want the project's warnings or a usage message")
			}
			switch {
			case tc.whole != nil:
				want, _ := wholeReport(t, root, tc.whole)
				if stdout != want {
					t.Errorf("stdout =\n%s\nwant exactly the report\n%s\nstderr:\n%s", stdout, want, stderr)
				}
			case tc.idsText:
				checkCoverageText(t, stdout, idsWant, "1/2 covered")
			case tc.idsJSON:
				var got jsonCoverage
				decodeOne(t, stdout, &got)
				if len(got.IDs) != 2 || got.IDs[0].ID != "DEMO-1-1" || got.IDs[1].ID != "DEMO-1-3" {
					t.Errorf("ids = %+v, want DEMO-1-1 and DEMO-1-3", got.IDs)
				}
			default:
				if stdout != "" {
					t.Errorf("stdout = %q, want empty\nstderr:\n%s", stdout, stderr)
				}
			}
		})
	}
}

// COVERAGE-2-1
func TestRun_ListedIDsReported(t *testing.T) {
	root := demoProject(t, severityConfig(config.SeverityError, config.SeverityError, config.SeverityError))
	ids := "demo-01-02,DEMO-1-INV-1,DEMO-1-3,demo-1-inv-02,DEMO-1-2,DEMO-1-1,DEMO-1-INV-1,DEMO-7-1"
	want := demoEntries("DEMO-1-2", "DEMO-1-INV-1", "DEMO-1-3", "DEMO-1-INV-2", "DEMO-1-1", "DEMO-7-1")
	for _, args := range [][]string{
		{"--ids", ids},
		{"--ids=" + ids},
		{"--ids", ids, "--format", "text"},
		{"--format", "text", "--ids", ids},
		{"--ids=" + ids, "--format=text"},
		{"--format=text", "--ids=" + ids},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, stdout, _ := run(t, root, args...)
			checkCoverageText(t, stdout, want, "3/6 covered")
		})
	}
}

// COVERAGE-2-1
func TestRun_ListedIDsTestOrder(t *testing.T) {
	root := multiFileProject(t, severityConfig(config.SeverityError, config.SeverityError, config.SeverityError))
	_, stdout, _ := run(t, root, "--ids", "DEMO-1-INV-1,DEMO-1-1,DEMO-1-2")
	want := []textEntry{
		{ID: "DEMO-1-INV-1", State: "covered", Tests: []string{
			earlyTestPath + ":23 TestEarlyInv",
			demoTestPath + ":13 TestBoth",
		}},
		{ID: "DEMO-1-1", State: "covered", Tests: []string{
			earlyTestPath + ":20 TestEarly",
			demoTestPath + ":6 TestOne",
			demoTestPath + ":13 TestBoth",
			lateTestPath + ":5 TestLate",
		}},
		{ID: "DEMO-1-2", State: "covered", Tests: []string{demoTestPath + ":9 TestTwo"}},
	}
	checkCoverageText(t, stdout, want, "3/3 covered")
}

// COVERAGE-2-1
func TestRun_ListedIDsTestLines(t *testing.T) {
	root := demoProject(t, severityConfig(config.SeverityError, config.SeverityError, config.SeverityError))
	_, stdout, _ := run(t, root, "--ids", "DEMO-1-2")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	var nonBlank []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonBlank = append(nonBlank, l)
		}
	}
	if len(nonBlank) != 3 {
		t.Fatalf("stdout =\n%s\nwant an ID line, one test line and the count line", stdout)
	}
	indent := func(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }
	if indent(nonBlank[1]) <= indent(nonBlank[0]) {
		t.Errorf("test line %q is not indented below ID line %q", nonBlank[1], nonBlank[0])
	}
	if got, want := strings.TrimSpace(nonBlank[1]), "pkg/demo_test.go:9 TestTwo"; got != want {
		t.Errorf("test line = %q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(nonBlank[2]), "1/1 covered"; got != want {
		t.Errorf("last line = %q, want %q", got, want)
	}
}

// COVERAGE-2-2
func TestRun_AllCoveredPasses(t *testing.T) {
	ids := "DEMO-1-1,DEMO-1-2,DEMO-1-INV-1"
	for _, format := range []string{"", "text", "json"} {
		name := "format " + format
		if format == "" {
			name = "default format"
		}
		t.Run(name, func(t *testing.T) {
			// DEMO-1-3 and DEMO-1-INV-2 are untested and DEMO-9-9 is
			// orphaned, all at error.
			root := demoProject(t, severityConfig(config.SeverityError, config.SeverityError, config.SeverityError))
			if _, exit := wholeReport(t, root, reporter.Text{}); exit != 1 {
				t.Fatalf("fixture: whole-repo exit = %d, want 1", exit)
			}
			args := []string{"--ids", ids}
			if format != "" {
				args = append(args, "--format", format)
			}
			code, stdout, stderr := run(t, root, args...)
			if code != 0 {
				t.Errorf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if format == "json" {
				var got jsonCoverage
				decodeOne(t, stdout, &got)
				if got.Covered != 3 || got.Total != 3 {
					t.Errorf("covered/total = %d/%d, want 3/3", got.Covered, got.Total)
				}
			} else {
				checkCoverageText(t, stdout, demoEntries("DEMO-1-1", "DEMO-1-2", "DEMO-1-INV-1"), "3/3 covered")
			}
		})
	}
}

// COVERAGE-2-3
func TestRun_UntestedFails(t *testing.T) {
	for _, sev := range []config.Severity{config.SeverityWarning, config.SeverityInfo, config.SeverityOff} {
		for _, tc := range []struct {
			name string
			ids  []string
		}{
			{"untested ac", []string{"DEMO-1-1", "DEMO-1-3", "DEMO-1-INV-1"}},
			{"untested invariant", []string{"DEMO-1-INV-2", "DEMO-1-2"}},
		} {
			t.Run(string(sev)+" "+tc.name, func(t *testing.T) {
				root := demoProject(t, severityConfig(sev, sev, config.SeverityOff))
				if _, exit := wholeReport(t, root, reporter.Text{}); exit != 0 {
					t.Fatalf("fixture: whole-repo exit = %d, want 0", exit)
				}
				code, stdout, stderr := run(t, root, "--ids", strings.Join(tc.ids, ","))
				if code != 1 {
					t.Errorf("exit = %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
				}
				want := demoEntries(tc.ids...)
				covered := 0
				for _, e := range want {
					if e.State == "covered" {
						covered++
					}
				}
				checkCoverageText(t, stdout, want, fmt.Sprintf("%d/%d covered", covered, len(want)))
			})
		}
	}
}

// COVERAGE-2-4
func TestRun_UnknownFails(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  string
		want []textEntry
		last string
	}{
		{"unknown ac", "DEMO-1-1,DEMO-7-1", demoEntries("DEMO-1-1", "DEMO-7-1"), "1/2 covered"},
		{"unknown invariant", "DEMO-1-INV-9,DEMO-1-2", demoEntries("DEMO-1-INV-9", "DEMO-1-2"), "1/2 covered"},
		{"unknown feature", "NOPE-1-1", demoEntries("NOPE-1-1"), "0/1 covered"},
		{"orphaned annotation id", "DEMO-9-9", demoEntries("DEMO-9-9"), "0/1 covered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := demoProject(t, severityConfig(config.SeverityOff, config.SeverityOff, config.SeverityOff))
			code, stdout, stderr := run(t, root, "--ids", tc.ids)
			if code != 1 {
				t.Errorf("exit = %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			checkCoverageText(t, stdout, tc.want, tc.last)
		})
	}
}

// COVERAGE-2-5
func TestRun_BadIDList(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  string
		want []string
	}{
		{"empty list", "", nil},
		{"empty item", "DEMO-1-1,,DEMO-1-2", nil},
		{"trailing empty item", "DEMO-1-1,", nil},
		{"malformed id", "DEMO-1-1,not-an-id", []string{"not-an-id"}},
		{"malformed invariant id", "DEMO-1-INV,DEMO-1-2", []string{"DEMO-1-INV"}},
		{"malformed id not normalized", "DEMO-1-1,demo-01-inv", []string{"demo-01-inv"}},
		{"requirement id", "DEMO-1-1,SLICE-8", []string{"SLICE-8"}},
		{"requirement id not normalized", "DEMO-1-1,slice-08", []string{"slice-08"}},
		{"defined requirement id", "DEMO-1", []string{"DEMO-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := demoProject(t, severityConfig(config.SeverityOff, config.SeverityOff, config.SeverityOff))
			for _, args := range [][]string{{"--ids", tc.ids}, {"--ids=" + tc.ids}} {
				code, stdout, stderr := run(t, root, args...)
				checkUsageError(t, code, stdout, stderr, tc.want...)
			}
		})
	}
}

// COVERAGE-2-6
func TestRun_JSONForListedIDs(t *testing.T) {
	root := multiFileProject(t, severityConfig(config.SeverityError, config.SeverityError, config.SeverityError))
	ids := "DEMO-1-1,demo-1-inv-1,DEMO-1-3,DEMO-1-INV-2,DEMO-7-1,OTHER-2-INV-1,DEMO-1-1"
	for _, args := range [][]string{
		{"--ids", ids, "--format", "json"},
		{"--format", "json", "--ids", ids},
		{"--ids=" + ids, "--format=json"},
		{"--format=json", "--ids=" + ids},
		{"--format=json", "--ids", ids},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			checkJSONForListedIDs(t, root, args)
		})
	}
}

func checkJSONForListedIDs(t *testing.T, root string, args []string) {
	t.Helper()
	code, stdout, stderr := run(t, root, args...)
	if code != 1 {
		t.Errorf("exit = %d, want 1\nstderr:\n%s", code, stderr)
	}

	var top json.RawMessage
	decodeOne(t, stdout, &top)
	if got, want := keys(t, top), []string{"covered", "ids", "total"}; !reflect.DeepEqual(got, want) {
		t.Errorf("top-level keys = %v, want %v", got, want)
	}
	var raw struct {
		IDs []json.RawMessage `json:"ids"`
	}
	if err := json.Unmarshal(top, &raw); err != nil {
		t.Fatal(err)
	}
	for _, e := range raw.IDs {
		if got, want := keys(t, e), []string{"covered", "id", "kind", "tests"}; !reflect.DeepEqual(got, want) {
			t.Errorf("entry keys = %v, want %v in %s", got, want, e)
		}
	}

	var got jsonCoverage
	if err := json.Unmarshal(top, &got); err != nil {
		t.Fatal(err)
	}
	if got.Covered != 2 || got.Total != 6 {
		t.Errorf("covered/total = %d/%d, want 2/6", got.Covered, got.Total)
	}

	type entry struct {
		ID      string
		Kind    string
		Covered bool
		Tests   []jsonCoverageTest
	}
	want := []entry{
		{"DEMO-1-1", "ac", true, []jsonCoverageTest{
			{earlyTestPath, 20, "TestEarly"},
			{demoTestPath, 6, "TestOne"},
			{demoTestPath, 13, "TestBoth"},
			{lateTestPath, 5, "TestLate"},
		}},
		{"DEMO-1-INV-1", "invariant", true, []jsonCoverageTest{
			{earlyTestPath, 23, "TestEarlyInv"},
			{demoTestPath, 13, "TestBoth"},
		}},
		{"DEMO-1-3", "ac", false, []jsonCoverageTest{}},
		{"DEMO-1-INV-2", "invariant", false, []jsonCoverageTest{}},
		{"DEMO-7-1", "unknown", false, []jsonCoverageTest{}},
		{"OTHER-2-INV-1", "unknown", false, []jsonCoverageTest{}},
	}
	var entries []entry
	for _, e := range got.IDs {
		if strings.TrimSpace(string(e.Tests)) == "null" {
			t.Errorf("%s: tests is null, want []", e.ID)
		}
		var rawTests []json.RawMessage
		if err := json.Unmarshal(e.Tests, &rawTests); err != nil {
			t.Fatalf("%s: tests: %v", e.ID, err)
		}
		tests := []jsonCoverageTest{}
		for _, rt := range rawTests {
			if got, want := keys(t, rt), []string{"file", "function", "line"}; !reflect.DeepEqual(got, want) {
				t.Errorf("%s: test keys = %v, want %v", e.ID, got, want)
			}
			var ct jsonCoverageTest
			if err := json.Unmarshal(rt, &ct); err != nil {
				t.Fatalf("%s: test: %v", e.ID, err)
			}
			tests = append(tests, ct)
		}
		entries = append(entries, entry{e.ID, e.Kind, e.Covered, tests})
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("ids = %+v\nwant %+v", entries, want)
	}
}

// COVERAGE-2-6
func TestRun_MarkdownForListedIDsIsUsageError(t *testing.T) {
	root := demoProject(t, severityConfig(config.SeverityOff, config.SeverityOff, config.SeverityOff))
	for _, args := range [][]string{
		{"--ids", "DEMO-1-1", "--format", "markdown"},
		{"--format", "markdown", "--ids", "DEMO-1-1"},
		{"--ids=DEMO-1-1", "--format=markdown"},
		{"--format=markdown", "--ids=DEMO-1-1"},
	} {
		code, stdout, stderr := run(t, root, args...)
		checkUsageError(t, code, stdout, stderr)
	}
}
