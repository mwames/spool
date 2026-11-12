package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/reporter"
)

// noIssuesLine is the text report's last line when it has no findings.
const noIssuesLine = "Full traceability — no issues found."

// annotatedTest returns a Go test file with one test per ID, annotated with
// it. It is built from lines so that the annotations aren't read as
// annotations of this file.
func annotatedTest(ids ...string) string {
	lines := []string{"package pkg", "", `import "testing"`, ""}
	for i, id := range ids {
		lines = append(lines, "// "+id, "func TestLinked"+string(rune('A'+i))+"(t *testing.T) {}", "")
	}
	return strings.Join(lines, "\n")
}

// lintConfig returns a .spool.yaml that scans Go test files and, unless sev
// is empty, sets the given severity field to sev.
func lintConfig(field, sev string) string {
	y := "test_patterns:\n  - \"**/*_test.go\"\n"
	if sev != "" {
		y += "severity:\n  " + field + ": " + sev + "\n"
	}
	return y
}

// ifaceReq names interface paths: LINT-2, written first, a single path
// ifaceProject writes only when asked to, LINT-1 a list holding a file and a
// directory that ifaceProject always writes and two paths, out of
// alphabetical order, it writes only when asked to. LINT-7, written
// deprecated with an active AC and so active (SPEC-4-4), and the draft
// LINT-3, written last, each name a path ifaceProject writes only when asked
// to. LINT-4, deprecated, LINT-5, superseded with an active AC, and LINT-6,
// written active with only a deprecated AC and so deprecated, each name a
// path it never writes. Every AC is written as Given/When/Then.
const ifaceReq = `feature: LINT

requirements:

  - id: LINT-2
    title: Single
    description: A requirement with one interface path.
    status: active
    interface: lib/missing.go
    acceptance_criteria:
      - id: LINT-2-1
        title: Two
        description: Given a path, when spool runs, then it is checked.

  - id: LINT-1
    title: Stubbed
    description: A requirement with interface paths.
    status: active
    interface:
      - stub/present.go
      - missing/dir
      - stub
      - ./stub/missing.go
    acceptance_criteria:
      - id: LINT-1-1
        title: One
        description: >
          Given a stub,
          when spool runs,
          then it is checked.

  - id: LINT-4
    title: Deprecated
    description: A deprecated requirement.
    status: deprecated
    interface: deprecated/missing.go
    acceptance_criteria:
      - id: LINT-4-1
        title: Four
        description: Given a deprecation, when spool runs, then it is skipped.

  - id: LINT-5
    title: Superseded
    description: A superseded requirement.
    status: superseded
    interface: superseded/missing.go
    acceptance_criteria:
      - id: LINT-5-1
        title: Five
        description: Given a supersession, when spool runs, then it is skipped.
        status: active

  - id: LINT-6
    title: Retired
    description: Written active, with only a deprecated AC.
    status: active
    interface: retired/missing.go
    acceptance_criteria:
      - id: LINT-6-1
        title: Six
        description: Given a retirement, when spool runs, then it is skipped.
        status: deprecated

  - id: LINT-7
    title: Revived
    description: Written deprecated, with an active AC.
    status: deprecated
    interface: revived/missing.go
    acceptance_criteria:
      - id: LINT-7-1
        title: Seven
        description: Given a revival, when spool runs, then it is checked.
        status: active

  - id: LINT-3
    title: Draft
    description: A draft requirement.
    status: draft
    interface: draft/missing.go
    acceptance_criteria:
      - id: LINT-3-1
        title: Three
        description: Given a draft, when spool runs, then it is checked.
`

// ifaceMissingText is the text report's "Missing Interfaces" entries for
// ifaceProject with its paths missing, in order: as the report's features,
// then as the requirements are in the .req file, then as the paths are
// written.
var ifaceMissingText = []string{
	"LINT-2: lib/missing.go",
	"LINT-1: missing/dir",
	"LINT-1: ./stub/missing.go",
	"LINT-7: revived/missing.go",
	"LINT-3: draft/missing.go",
}

type idPath struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

var ifaceMissingJSON = []idPath{
	{"LINT-2", "lib/missing.go"},
	{"LINT-1", "missing/dir"},
	{"LINT-1", "./stub/missing.go"},
	{"LINT-7", "revived/missing.go"},
	{"LINT-3", "draft/missing.go"},
}

// ifaceProject writes ifaceReq with a test for each active AC. Unless
// present, the interface paths of ifaceMissingText don't exist.
func ifaceProject(t *testing.T, spoolYAML string, present bool) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".spool.yaml":      spoolYAML,
		".spool/lint.req":  ifaceReq,
		"stub/present.go":  "package stub\n",
		"pkg/lint_test.go": annotatedTest("LINT-1-1", "LINT-2-1", "LINT-7-1"),
	}
	if present {
		files["stub/missing.go"] = "package stub\n"
		files["missing/dir/keep.txt"] = "\n"
		files["lib/missing.go"] = "package lib\n"
		files["revived/missing.go"] = "package revived\n"
		files["draft/missing.go"] = "package draft\n"
	}
	writeFiles(t, root, files)
	return root
}

// formReq has ACs written as Given/When/Then and ACs that aren't, in an
// active requirement and out of ID order, among them a draft AC and a
// deprecated one, and an invariant that isn't. FORM-2 is superseded with an
// active AC that isn't, and FORM-3, written deprecated, has an active AC
// that isn't and an AC that isn't and inherits deprecated. formOtherReq is a
// second feature, in a file read before formReq's, with an active AC that
// isn't and, last, a draft requirement with an AC that isn't.
const formReq = `feature: FORM

requirements:

  - id: FORM-1
    title: Formed
    description: A requirement.
    status: active
    acceptance_criteria:
      - id: FORM-1-1
        title: Formed AC
        description: >
          Given a form,
          when it is checked,
          then it passes.
      - id: FORM-1-3
        title: No When
        description: Given a form, then it fails.
      - id: FORM-1-2
        title: Unformed AC
        description: Spool reports it.
      - id: FORM-1-4
        title: Draft Unformed
        description: A draft AC is reported.
        status: draft
      - id: FORM-1-5
        title: Deprecated Unformed
        description: A deprecated AC is skipped.
        status: deprecated
    invariants:
      - id: FORM-1-INV-1
        title: Holds
        description: It always holds.

  - id: FORM-2
    title: Superseded
    description: A superseded requirement.
    status: superseded
    acceptance_criteria:
      - id: FORM-2-1
        title: Superseded AC
        description: Not checked.
        status: active

  - id: FORM-3
    title: Revived
    description: Written deprecated, with an active AC.
    status: deprecated
    acceptance_criteria:
      - id: FORM-3-1
        title: Revived Unformed
        description: A revived AC is reported.
        status: active
      - id: FORM-3-2
        title: Retired Unformed
        description: A retired AC is skipped.
`

const formOtherReq = `feature: OTHER

requirements:

  - id: OTHER-1
    title: Other
    description: Another requirement.
    status: active
    acceptance_criteria:
      - id: OTHER-1-1
        title: Other Unformed
        description: When x, given y, then z.

  - id: OTHER-2
    title: Other Draft
    description: A draft requirement.
    status: draft
    acceptance_criteria:
      - id: OTHER-2-1
        title: Draft Requirement Unformed
        description: An AC of a draft requirement is reported.
`

// formFixes rewrites the ACs of formReq and formOtherReq that are reported
// as not Given/When/Then so that they are.
var formFixes = strings.NewReplacer(
	"description: Spool reports it.", "description: Given a form, when it is checked, then Spool reports it.",
	"description: Given a form, then it fails.", "description: Given a form, when it is checked, then it fails.",
	"description: When x, given y, then z.", "description: Given x, when y, then z.",
	"description: A draft AC is reported.", "description: Given a draft AC, when it is checked, then it passes.",
	"description: A revived AC is reported.", "description: Given a revived AC, when it is checked, then it passes.",
	"description: An AC of a draft requirement is reported.", "description: Given a draft requirement, when it is checked, then it passes.",
)

// formNotGWTText is the text report's "Not Given/When/Then" entries for
// formProject, in order: as the report's features, then as the requirements
// are in the .req file, then as the ACs are written.
var formNotGWTText = []string{
	"FORM-1-3: No When",
	"FORM-1-2: Unformed AC",
	"FORM-1-4: Draft Unformed",
	"FORM-3-1: Revived Unformed",
	"OTHER-1-1: Other Unformed",
	"OTHER-2-1: Draft Requirement Unformed",
}

type idFeature struct {
	ID      string `json:"id"`
	Feature string `json:"feature"`
}

var formNotGWTJSON = []idFeature{
	{"FORM-1-3", "FORM"},
	{"FORM-1-2", "FORM"},
	{"FORM-1-4", "FORM"},
	{"FORM-3-1", "FORM"},
	{"OTHER-1-1", "OTHER"},
	{"OTHER-2-1", "OTHER"},
}

// formProject writes formReq and formOtherReq with a test for each active AC
// and invariant. When formed, every AC is written as Given/When/Then.
func formProject(t *testing.T, spoolYAML string, formed bool) string {
	t.Helper()
	root := t.TempDir()
	form, other := formReq, formOtherReq
	if formed {
		form, other = formFixes.Replace(form), formFixes.Replace(other)
	}
	writeFiles(t, root, map[string]string{
		".spool.yaml":        spoolYAML,
		".spool/form.req":    form,
		".spool/a_other.req": other,
		"pkg/form_test.go":   annotatedTest("FORM-1-1", "FORM-1-2", "FORM-1-3", "FORM-1-INV-1", "FORM-3-1", "OTHER-1-1"),
	})
	return root
}

// textSection returns the trimmed entries listed under the text report line
// that reads heading: the lines after it up to a blank line or the end,
// skipping an underline of dashes or equals signs. ok is false when there is
// no such heading.
func textSection(out, heading string) (entries []string, ok bool) {
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) != heading {
			continue
		}
		for _, e := range lines[i+1:] {
			e = strings.TrimSpace(e)
			if e == "" {
				break
			}
			if strings.Trim(e, "-=") == "" {
				continue
			}
			entries = append(entries, e)
		}
		return entries, true
	}
	return nil, false
}

// markdownSection returns the lines under the markdown heading that reads
// heading, up to the next heading or the end. ok is false when there is no
// such heading.
func markdownSection(out, heading string) (lines []string, ok bool) {
	all := strings.Split(out, "\n")
	for i, l := range all {
		if !strings.HasPrefix(l, "#") || strings.TrimSpace(strings.TrimLeft(l, "#")) != heading {
			continue
		}
		for _, s := range all[i+1:] {
			if strings.HasPrefix(s, "#") {
				break
			}
			lines = append(lines, s)
		}
		return lines, true
	}
	return nil, false
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

// jsonList returns the report's top-level key as a list of raw objects,
// failing the test if it is missing, null or not a list, and checks that
// each object has exactly the wanted keys.
func jsonList(t *testing.T, out, key string, wantKeys []string) []json.RawMessage {
	t.Helper()
	var top map[string]json.RawMessage
	decodeOne(t, out, &top)
	raw, ok := top[key]
	if !ok {
		t.Fatalf("report has no %q key:\n%s", key, out)
	}
	if strings.TrimSpace(string(raw)) == "null" {
		t.Fatalf("%q is null, want a list", key)
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("%q: %v\n%s", key, err, raw)
	}
	for _, e := range list {
		if got := keys(t, e); !reflect.DeepEqual(got, wantKeys) {
			t.Errorf("%q entry keys = %v, want %v in %s", key, got, wantKeys, e)
		}
	}
	return list
}

// SPEC-1-3
func TestRun_MissingInterfacesText(t *testing.T) {
	root := ifaceProject(t, lintConfig("", ""), false)
	_, stdout, stderr := run(t, root)
	got, ok := textSection(stdout, "Missing Interfaces")
	if !ok {
		t.Fatalf("no \"Missing Interfaces\" heading in:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !reflect.DeepEqual(got, ifaceMissingText) {
		t.Errorf("Missing Interfaces entries = %q, want %q\n%s", got, ifaceMissingText, stdout)
	}
}

// SPEC-1-3
func TestRun_MissingInterfacesMarkdown(t *testing.T) {
	root := ifaceProject(t, lintConfig("", ""), false)
	_, stdout, _ := run(t, root, "--format", "markdown")
	section, ok := markdownSection(stdout, "Missing Interfaces")
	if !ok {
		t.Fatalf("no \"Missing Interfaces\" heading in:\n%s", stdout)
	}
	last := -1
	for _, f := range ifaceMissingJSON {
		at := -1
		for i, l := range section {
			if strings.Contains(l, f.ID) && strings.Contains(l, f.Path) {
				at = i
				break
			}
		}
		if at < 0 {
			t.Errorf("no line under \"Missing Interfaces\" gives %s and %s:\n%s", f.ID, f.Path, stdout)
			continue
		}
		if at <= last {
			t.Errorf("%s %s is listed before the finding it follows, want them in order %+v:\n%s", f.ID, f.Path, ifaceMissingJSON, stdout)
		}
		last = at
	}
	for _, l := range section {
		for _, p := range []string{"stub/present.go", "deprecated/missing.go", "superseded/missing.go", "retired/missing.go", "LINT-4", "LINT-5", "LINT-6"} {
			if strings.Contains(l, p) {
				t.Errorf("line %q under \"Missing Interfaces\" names %s, which gives no finding", l, p)
			}
		}
	}
}

// SPEC-1-3
func TestRun_MissingInterfacesJSON(t *testing.T) {
	root := ifaceProject(t, lintConfig("", ""), false)
	_, stdout, _ := run(t, root, "--format", "json")
	var got []idPath
	for _, raw := range jsonList(t, stdout, "missingInterfaces", []string{"id", "path"}) {
		var f idPath
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		got = append(got, f)
	}
	if !reflect.DeepEqual(got, ifaceMissingJSON) {
		t.Errorf("missingInterfaces = %+v\nwant %+v", got, ifaceMissingJSON)
	}
}

// SPEC-1-3
func TestRun_NoMissingInterfacesJSON(t *testing.T) {
	for name, root := range map[string]string{
		"all paths present": ifaceProject(t, lintConfig("", ""), true),
		"no interfaces":     demoProject(t, lintConfig("", "")),
	} {
		t.Run(name, func(t *testing.T) {
			_, stdout, _ := run(t, root, "--format", "json")
			if list := jsonList(t, stdout, "missingInterfaces", []string{"id", "path"}); len(list) != 0 {
				t.Errorf("missingInterfaces = %s, want []", list)
			}
		})
	}
}

// SPEC-1-3
func TestRun_NoMissingInterfacesHeading(t *testing.T) {
	for name, root := range map[string]string{
		"all paths present": ifaceProject(t, lintConfig("", ""), true),
		"no interfaces":     demoProject(t, lintConfig("", "")),
	} {
		t.Run(name, func(t *testing.T) {
			_, stdout, _ := run(t, root)
			if got, ok := textSection(stdout, "Missing Interfaces"); ok {
				t.Errorf("text report has a \"Missing Interfaces\" heading with %q, want none:\n%s", got, stdout)
			}
			_, stdout, _ = run(t, root, "--format", "markdown")
			if _, ok := markdownSection(stdout, "Missing Interfaces"); ok {
				t.Errorf("markdown report has a \"Missing Interfaces\" heading, want none:\n%s", stdout)
			}
		})
	}
}

// SPEC-1-4
func TestRun_MissingInterfaceSeverity(t *testing.T) {
	for _, tc := range []struct {
		sev      string
		wantExit int
	}{
		{"error", 1},
		{"warning", 0},
		{"info", 0},
		{"off", 0},
		{"", 1},
	} {
		name := tc.sev
		if name == "" {
			name = "not set"
		}
		t.Run(name, func(t *testing.T) {
			yaml := lintConfig("missing_interface", tc.sev)
			if code, stdout, stderr := run(t, ifaceProject(t, yaml, true)); code != 0 {
				t.Fatalf("fixture: exit with every path present = %d, want 0\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			root := ifaceProject(t, yaml, false)
			code, stdout, stderr := run(t, root)
			if code != tc.wantExit {
				t.Errorf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.wantExit, stdout, stderr)
			}
			if got, _ := textSection(stdout, "Missing Interfaces"); !reflect.DeepEqual(got, ifaceMissingText) {
				t.Errorf("Missing Interfaces entries = %q, want %q\n%s", got, ifaceMissingText, stdout)
			}
			code, stdout, _ = run(t, root, "--format", "json")
			if code != tc.wantExit {
				t.Errorf("json: exit = %d, want %d", code, tc.wantExit)
			}
			if list := jsonList(t, stdout, "missingInterfaces", []string{"id", "path"}); len(list) != len(ifaceMissingJSON) {
				t.Errorf("json: missingInterfaces = %s, want %d findings", list, len(ifaceMissingJSON))
			}
		})
	}
}

// SPEC-3-2
func TestRun_NotGivenWhenThenText(t *testing.T) {
	root := formProject(t, lintConfig("", ""), false)
	_, stdout, stderr := run(t, root)
	got, ok := textSection(stdout, "Not Given/When/Then")
	if !ok {
		t.Fatalf("no \"Not Given/When/Then\" heading in:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !reflect.DeepEqual(got, formNotGWTText) {
		t.Errorf("Not Given/When/Then entries = %q, want %q\n%s", got, formNotGWTText, stdout)
	}
}

// SPEC-3-2
func TestRun_NotGivenWhenThenMarkdown(t *testing.T) {
	root := formProject(t, lintConfig("", ""), false)
	_, stdout, _ := run(t, root, "--format", "markdown")
	section, ok := markdownSection(stdout, "Not Given/When/Then")
	if !ok {
		t.Fatalf("no \"Not Given/When/Then\" heading in:\n%s", stdout)
	}
	text := strings.Join(section, "\n")
	last := -1
	for _, f := range formNotGWTJSON {
		at := -1
		for i, l := range section {
			if strings.Contains(l, f.ID) {
				at = i
				break
			}
		}
		if at < 0 {
			t.Errorf("no line under \"Not Given/When/Then\" gives %s:\n%s", f.ID, stdout)
			continue
		}
		if at <= last {
			t.Errorf("%s is listed before the finding it follows, want them in order %+v:\n%s", f.ID, formNotGWTJSON, stdout)
		}
		last = at
	}
	for _, id := range []string{"FORM-1-1", "FORM-1-5", "FORM-1-INV-1", "FORM-2-1", "FORM-3-2"} {
		if strings.Contains(text, id) {
			t.Errorf("%s is listed under \"Not Given/When/Then\", want no finding:\n%s", id, stdout)
		}
	}
}

// SPEC-3-2
func TestRun_NotGivenWhenThenJSON(t *testing.T) {
	root := formProject(t, lintConfig("", ""), false)
	_, stdout, _ := run(t, root, "--format", "json")
	var got []idFeature
	for _, raw := range jsonList(t, stdout, "notGivenWhenThen", []string{"feature", "id"}) {
		var f idFeature
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		got = append(got, f)
	}
	if !reflect.DeepEqual(got, formNotGWTJSON) {
		t.Errorf("notGivenWhenThen = %+v\nwant %+v", got, formNotGWTJSON)
	}
}

// SPEC-3-2
func TestRun_NoNotGivenWhenThenJSON(t *testing.T) {
	for name, root := range map[string]string{
		"all formed": formProject(t, lintConfig("", ""), true),
		"demo":       demoProject(t, lintConfig("", "")),
	} {
		t.Run(name, func(t *testing.T) {
			_, stdout, _ := run(t, root, "--format", "json")
			if list := jsonList(t, stdout, "notGivenWhenThen", []string{"feature", "id"}); len(list) != 0 {
				t.Errorf("notGivenWhenThen = %s, want []", list)
			}
		})
	}
}

// SPEC-3-2
func TestRun_NoNotGivenWhenThenHeading(t *testing.T) {
	for name, root := range map[string]string{
		"all formed": formProject(t, lintConfig("", ""), true),
		"demo":       demoProject(t, lintConfig("", "")),
	} {
		t.Run(name, func(t *testing.T) {
			_, stdout, _ := run(t, root)
			if got, ok := textSection(stdout, "Not Given/When/Then"); ok {
				t.Errorf("text report has a \"Not Given/When/Then\" heading with %q, want none:\n%s", got, stdout)
			}
			_, stdout, _ = run(t, root, "--format", "markdown")
			if _, ok := markdownSection(stdout, "Not Given/When/Then"); ok {
				t.Errorf("markdown report has a \"Not Given/When/Then\" heading, want none:\n%s", stdout)
			}
		})
	}
}

// SPEC-3-3
func TestRun_GivenWhenThenSeverity(t *testing.T) {
	for _, tc := range []struct {
		sev      string
		wantExit int
	}{
		{"error", 1},
		{"warning", 0},
		{"info", 0},
		{"off", 0},
		{"", 1},
	} {
		name := tc.sev
		if name == "" {
			name = "not set"
		}
		t.Run(name, func(t *testing.T) {
			yaml := lintConfig("given_when_then", tc.sev)
			if code, stdout, stderr := run(t, formProject(t, yaml, true)); code != 0 {
				t.Fatalf("fixture: exit with every AC formed = %d, want 0\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			root := formProject(t, yaml, false)
			code, stdout, stderr := run(t, root)
			if code != tc.wantExit {
				t.Errorf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.wantExit, stdout, stderr)
			}
			if got, _ := textSection(stdout, "Not Given/When/Then"); !reflect.DeepEqual(got, formNotGWTText) {
				t.Errorf("Not Given/When/Then entries = %q, want %q\n%s", got, formNotGWTText, stdout)
			}
			code, stdout, _ = run(t, root, "--format", "json")
			if code != tc.wantExit {
				t.Errorf("json: exit = %d, want %d", code, tc.wantExit)
			}
			if list := jsonList(t, stdout, "notGivenWhenThen", []string{"feature", "id"}); len(list) != len(formNotGWTJSON) {
				t.Errorf("json: notGivenWhenThen = %s, want %d findings", list, len(formNotGWTJSON))
			}
		})
	}
}

// SPEC-3-5
func TestRun_LintFindingsAreIssues(t *testing.T) {
	endsClean := func(out string) bool {
		return strings.HasSuffix(strings.TrimRight(out, "\n"), noIssuesLine)
	}
	for _, sev := range []string{"", "off"} {
		for _, tc := range []struct {
			name    string
			project func(t *testing.T, spoolYAML string, clean bool) string
			field   string
		}{
			{"missing interface", ifaceProject, "missing_interface"},
			{"not given/when/then", formProject, "given_when_then"},
		} {
			t.Run(tc.name+" severity "+sev, func(t *testing.T) {
				yaml := lintConfig(tc.field, sev)
				if _, stdout, _ := run(t, tc.project(t, yaml, true)); !endsClean(stdout) {
					t.Fatalf("fixture: report without the finding does not end with %q:\n%s", noIssuesLine, stdout)
				}
				_, stdout, _ := run(t, tc.project(t, yaml, false))
				if endsClean(stdout) {
					t.Errorf("report ends with %q despite the finding:\n%s", noIssuesLine, stdout)
				}
			})
		}
	}
}

// reqsAlpha and reqsBeta are the two features of requirementsProject.
// ALPHA lists its active requirements out of ID order, with a deprecated one
// among them.
const reqsAlpha = `feature: ALPHA

requirements:

  - id: ALPHA-3
    title: Alpha Three
    description: Interfaces and out_of_scope as lists.
    status: active
    interface:
      - ./alpha/stub.go
      - alpha
    out_of_scope:
      - Globs in paths
      - Checking the stub
    acceptance_criteria:
      - id: ALPHA-3-1
        title: Three
        description: Given a, when b, then c.

  - id: ALPHA-1
    title: Alpha One
    description: A single interface path.
    status: active
    interface: alpha/one.go
    acceptance_criteria:
      - id: ALPHA-1-1
        title: One
        description: Given a, when b, then c.

  - id: ALPHA-2
    title: Alpha Deprecated
    description: Not active.
    status: deprecated
    interface: alpha/two.go
    out_of_scope: Nothing
    acceptance_criteria:
      - id: ALPHA-2-1
        title: Two
        description: Given a, when b, then c.

  - id: ALPHA-4
    title: Alpha Four
    description: A single out_of_scope item.
    status: active
    out_of_scope: Only this
    acceptance_criteria:
      - id: ALPHA-4-1
        title: Four
        description: Given a, when b, then c.

  - id: ALPHA-5
    title: Alpha Five
    description: Written active, with a draft AC; counted, and folded draft.
    status: active
    acceptance_criteria:
      - id: ALPHA-5-1
        title: Five
        description: Given a, when b, then c.
      - id: ALPHA-5-2
        title: Five Draft
        description: Given a, when b, then c.
        status: draft

  - id: ALPHA-6
    title: Alpha Draft
    description: A draft; not counted.
    status: draft
    acceptance_criteria:
      - id: ALPHA-6-1
        title: Six
        description: Given a, when b, then c.

  - id: ALPHA-7
    title: Alpha Seven
    description: Written deprecated, with an active AC; counted, and folded active.
    status: deprecated
    out_of_scope: Seven
    acceptance_criteria:
      - id: ALPHA-7-1
        title: Seven
        description: Given a, when b, then c.
        status: active
`

const reqsBeta = `feature: BETA

requirements:

  - id: BETA-1
    title: Beta One
    description: Empty out_of_scope.
    status: active
    out_of_scope: []
    acceptance_criteria:
      - id: BETA-1-1
        title: One
        description: Given a, when b, then c.

  - id: BETA-2
    title: Beta Two
    description: Active with no items; counted.
    status: active
    acceptance_criteria: []

  - id: BETA-3
    title: Beta Three
    description: Draft with no items; not counted.
    status: draft

  - id: BETA-4
    title: Beta Four
    description: Written active, with only a deprecated AC; not counted.
    status: active
    acceptance_criteria:
      - id: BETA-4-1
        title: Four
        description: Given a, when b, then c.
        status: deprecated

  - id: BETA-5
    title: Beta Five
    description: Superseded with an active AC; not counted.
    status: superseded
    acceptance_criteria:
      - id: BETA-5-1
        title: Five
        description: Given a, when b, then c.
        status: active
`

type jsonRequirement struct {
	ID         string   `json:"id"`
	Feature    string   `json:"feature"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Interface  []string `json:"interface"`
	OutOfScope []string `json:"outOfScope"`
}

// SPEC-2-2
// SPEC-4-4
// SPEC-4-6
func TestRun_RequirementsJSON(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".spool.yaml":      lintConfig("", ""),
		".spool/beta.req":  reqsBeta,
		".spool/alpha.req": reqsAlpha,
		"alpha/stub.go":    "package alpha\n",
		"pkg/reqs_test.go": annotatedTest("ALPHA-3-1", "ALPHA-1-1", "ALPHA-4-1", "ALPHA-5-1", "ALPHA-7-1", "BETA-1-1"),
	})
	_, stdout, _ := run(t, root, "--format", "json")

	byFeature := map[string][]jsonRequirement{
		"ALPHA": {
			{"ALPHA-3", "ALPHA", "Alpha Three", "active", []string{"./alpha/stub.go", "alpha"}, []string{"Globs in paths", "Checking the stub"}},
			{"ALPHA-1", "ALPHA", "Alpha One", "active", []string{"alpha/one.go"}, []string{}},
			{"ALPHA-4", "ALPHA", "Alpha Four", "active", []string{}, []string{"Only this"}},
			{"ALPHA-5", "ALPHA", "Alpha Five", "draft", []string{}, []string{}},
			{"ALPHA-7", "ALPHA", "Alpha Seven", "active", []string{}, []string{"Seven"}},
		},
		"BETA": {
			{"BETA-1", "BETA", "Beta One", "active", []string{}, []string{}},
			{"BETA-2", "BETA", "Beta Two", "active", []string{}, []string{}},
		},
	}
	var report struct {
		Features []struct {
			Name string `json:"name"`
		} `json:"features"`
	}
	decodeOne(t, stdout, &report)
	var want []jsonRequirement
	for _, f := range report.Features {
		want = append(want, byFeature[f.Name]...)
	}
	if len(report.Features) != 2 {
		t.Fatalf("fixture: features = %+v, want ALPHA and BETA", report.Features)
	}

	var got []jsonRequirement
	for _, raw := range jsonList(t, stdout, "requirements", []string{"feature", "id", "interface", "outOfScope", "status", "title"}) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"interface", "outOfScope"} {
			if strings.TrimSpace(string(fields[k])) == "null" {
				t.Errorf("%s is null, want [] in %s", k, raw)
			}
		}
		var r jsonRequirement
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatal(err)
		}
		if r.Interface == nil {
			r.Interface = []string{}
		}
		if r.OutOfScope == nil {
			r.OutOfScope = []string{}
		}
		got = append(got, r)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("requirements = %+v\nwant %+v", got, want)
	}
}

// SPEC-2-2
func TestRun_JSONOtherKeysUnchanged(t *testing.T) {
	root := demoProject(t, lintConfig("", ""))
	_, stdout, _ := run(t, root, "--format", "json")
	want, _ := wholeReport(t, root, reporter.JSON{})

	oldKeys := []string{
		"features", "linkedACs", "linkedInvariants", "orphaned", "orphanedCount",
		"totalACs", "totalInvariants", "totalRequirements", "untested",
		"untestedACs", "untestedInvariantList", "untestedInvariants",
	}
	wantKeys := sorted(append(append([]string(nil), oldKeys...), "missingInterfaces", "notGivenWhenThen", "requirements"))
	if got := keys(t, []byte(stdout)); !reflect.DeepEqual(got, wantKeys) {
		t.Errorf("top-level keys = %v, want %v", got, wantKeys)
	}

	var gotTop, wantTop map[string]json.RawMessage
	decodeOne(t, stdout, &gotTop)
	decodeOne(t, want, &wantTop)
	for _, k := range oldKeys {
		var g, w bytes.Buffer
		if err := json.Compact(&g, gotTop[k]); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		if err := json.Compact(&w, wantTop[k]); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		if g.String() != w.String() {
			t.Errorf("%s = %s, want %s", k, g.String(), w.String())
		}
	}
}

// invReq is the project of the lint invariant. When dirty, its interface
// paths are missing and three of its ACs, one of them draft, aren't
// Given/When/Then; otherwise they are present and formed. INV-1-2 and
// INV-1-INV-2 are untested either way.
const invReq = `feature: INV

requirements:

  - id: INV-1
    title: Inv
    description: A requirement.
    status: active
    interface:
      - stub/a.go
      - stub/dir
    acceptance_criteria:
      - id: INV-1-1
        title: One
        description: FIRST
      - id: INV-1-2
        title: Two
        description: SECOND
    invariants:
      - id: INV-1-INV-1
        title: Inv One
        description: It holds.
      - id: INV-1-INV-2
        title: Inv Two
        description: It holds too.

  - id: INV-2
    title: Draft
    description: A draft.
    status: draft
    interface: stub/draft.go
    acceptance_criteria:
      - id: INV-2-1
        title: Draft AC
        description: THIRD
`

func invProject(t *testing.T, dirty bool) string {
	t.Helper()
	root := t.TempDir()
	first, second, third := "Given a, when b, then c.", "Given a, when b, then d.", "Given a, when b, then e."
	files := map[string]string{
		".spool.yaml":     lintConfig("", ""),
		"pkg/inv_test.go": annotatedTest("INV-1-1", "INV-1-INV-1", "INV-9-9"),
	}
	if dirty {
		first, second, third = "Spool reports it.", "Given a, then d.", "Not checked."
	} else {
		files["stub/a.go"] = "package stub\n"
		files["stub/dir/keep.txt"] = "\n"
		files["stub/draft.go"] = "package stub\n"
	}
	files[".spool/inv.req"] = strings.NewReplacer("FIRST", first, "SECOND", second, "THIRD", third).Replace(invReq)
	writeFiles(t, root, files)
	return root
}

// SPEC-3-INV-1
func TestRun_LintLeavesCoverageAlone(t *testing.T) {
	dirty, clean := invProject(t, true), invProject(t, false)

	_, dirtyJSON, _ := run(t, dirty, "--format", "json")
	_, cleanJSON, _ := run(t, clean, "--format", "json")
	if list := jsonList(t, dirtyJSON, "missingInterfaces", []string{"id", "path"}); len(list) != 3 {
		t.Fatalf("fixture: dirty missingInterfaces = %s, want 3 findings", list)
	}
	if list := jsonList(t, dirtyJSON, "notGivenWhenThen", []string{"feature", "id"}); len(list) != 3 {
		t.Fatalf("fixture: dirty notGivenWhenThen = %s, want 3 findings", list)
	}

	type counts struct {
		TotalRequirements  int             `json:"totalRequirements"`
		TotalACs           int             `json:"totalACs"`
		LinkedACs          int             `json:"linkedACs"`
		UntestedACs        int             `json:"untestedACs"`
		OrphanedCount      int             `json:"orphanedCount"`
		TotalInvariants    int             `json:"totalInvariants"`
		LinkedInvariants   int             `json:"linkedInvariants"`
		UntestedInvariants int             `json:"untestedInvariants"`
		Features           json.RawMessage `json:"features"`
		Untested           []struct {
			ID string `json:"id"`
		} `json:"untested"`
		UntestedInvariantList []struct {
			ID string `json:"id"`
		} `json:"untestedInvariantList"`
		Orphaned []struct {
			ID   string `json:"id"`
			Line int    `json:"line"`
		} `json:"orphaned"`
	}
	var d, c counts
	decodeOne(t, dirtyJSON, &d)
	decodeOne(t, cleanJSON, &c)
	var df, cf bytes.Buffer
	if err := json.Compact(&df, d.Features); err != nil {
		t.Fatal(err)
	}
	if err := json.Compact(&cf, c.Features); err != nil {
		t.Fatal(err)
	}
	d.Features, c.Features = df.Bytes(), cf.Bytes()
	if !reflect.DeepEqual(d, c) {
		t.Errorf("coverage with lint findings = %+v\nwant as without them %+v", d, c)
	}

	_, dirtyText, _ := run(t, dirty)
	_, cleanText, _ := run(t, clean)
	totals := func(out string) []string {
		var ls []string
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "Totals:") || strings.HasPrefix(l, "Invariant totals:") {
				ls = append(ls, l)
			}
		}
		return ls
	}
	if dt, ct := totals(dirtyText), totals(cleanText); len(ct) != 2 || !reflect.DeepEqual(dt, ct) {
		t.Errorf("text totals with lint findings = %q, want as without them %q", dt, ct)
	}

	for _, args := range [][]string{
		{"--ids", "INV-1-1,INV-1-INV-1"},
		{"--ids", "INV-1-1,INV-1-INV-1", "--format", "json"},
		{"--ids", "INV-1-1,INV-1-2,INV-1-INV-1,INV-1-INV-2,INV-2-1,INV-7-1"},
		{"--ids", "INV-1-1,INV-1-2,INV-1-INV-1,INV-1-INV-2,INV-2-1,INV-7-1", "--format", "json"},
	} {
		dCode, dOut, _ := run(t, dirty, args...)
		cCode, cOut, _ := run(t, clean, args...)
		if dOut != cOut {
			t.Errorf("%v: stdout with lint findings =\n%s\nwant as without them\n%s", args, dOut, cOut)
		}
		if dCode != cCode {
			t.Errorf("%v: exit with lint findings = %d, want as without them %d", args, dCode, cCode)
		}
	}
}
