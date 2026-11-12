package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/config"
)

// hashReq is the spec spool hash is run against. HASH-1 has whitespace to
// collapse in its title, description and an out_of_scope item, and two
// interface paths out of alphabetical order. HASH-1-10 has a multi-byte
// title and a literal block description. HASH-2 is a draft with no
// description, interface or out_of_scope. HASH-3 gives its interface and
// out_of_scope as single strings. HASH-1-3 and HASH-1-INV-2 are never listed.
const hashReq = `feature: HASH

requirements:

  - id: HASH-1
    title: "  Spaced\t Title  "
    description: >
      A requirement
      over two lines.
    status: active
    deciders:
      - Ada
    consulted: []
    date: 2026-11-01
    rationale: Because.
    superseded_by:
    interface:
      - internal/b.go
      - internal/a.go
    out_of_scope:
      - "Not  this\tone "
      - Nor that
    acceptance_criteria:
      - id: HASH-1-2
        title: Two
        description: Given x, when y, then z.
      - id: HASH-1-10
        title: Ten — café
        description: |
          Literal
            block.
      - id: HASH-1-3
        title: Unlisted
        description: Given an unlisted AC, when hashed, then it is left out.
    invariants:
      - id: HASH-1-INV-1
        title: Inv
        description: Always holds.
      - id: HASH-1-INV-2
        title: Unlisted Inv
        description: Never listed.

  - id: HASH-2
    title: Bare
    status: draft
    acceptance_criteria:
      - id: HASH-2-1
        title: Bare AC
        description: Given nothing, when hashed, then counts are zero.

  - id: HASH-3
    title: Single
    description: One path.
    status: active
    interface: lib/one.go
    out_of_scope: Just this
    invariants:
      - id: HASH-3-INV-2
        title: Single Inv
        description: "Tab\there."
`

// hashIDs lists the five hashed items out of order, with a repeat, lower
// case and leading zeros.
const hashIDs = "hash-1-010,HASH-1-INV-1,HASH-3-INV-02,HASH-1-2,HASH-2-1,HASH-1-2"

// hashCanonical is the canonical form of hashIDs against hashReq, written
// out by hand (SPEC-5-2). Records are ordered by ID in byte order:
// "HASH-1-10" < "HASH-1-2" < "HASH-1-INV-1" < "HASH-2-1" < "HASH-3-INV-2".
const hashCanonical = "" +
	// Record 1.
	"9:HASH-1-10\n" +
	"13:Ten — café\n" +
	"14:Literal block.\n" +
	"6:HASH-1\n" +
	"12:Spaced Title\n" +
	"29:A requirement over two lines.\n" +
	"1:2\n" +
	"13:internal/b.go\n" +
	"13:internal/a.go\n" +
	"1:2\n" +
	"12:Not this one\n" +
	"8:Nor that\n" +
	// Record 2.
	"8:HASH-1-2\n" +
	"3:Two\n" +
	"24:Given x, when y, then z.\n" +
	"6:HASH-1\n" +
	"12:Spaced Title\n" +
	"29:A requirement over two lines.\n" +
	"1:2\n" +
	"13:internal/b.go\n" +
	"13:internal/a.go\n" +
	"1:2\n" +
	"12:Not this one\n" +
	"8:Nor that\n" +
	// Record 3.
	"12:HASH-1-INV-1\n" +
	"3:Inv\n" +
	"13:Always holds.\n" +
	"6:HASH-1\n" +
	"12:Spaced Title\n" +
	"29:A requirement over two lines.\n" +
	"1:2\n" +
	"13:internal/b.go\n" +
	"13:internal/a.go\n" +
	"1:2\n" +
	"12:Not this one\n" +
	"8:Nor that\n" +
	// Record 4.
	"8:HASH-2-1\n" +
	"7:Bare AC\n" +
	"49:Given nothing, when hashed, then counts are zero.\n" +
	"6:HASH-2\n" +
	"4:Bare\n" +
	"0:\n" +
	"1:0\n" +
	"1:0\n" +
	// Record 5.
	"12:HASH-3-INV-2\n" +
	"10:Single Inv\n" +
	"9:Tab here.\n" +
	"6:HASH-3\n" +
	"6:Single\n" +
	"9:One path.\n" +
	"1:1\n" +
	"10:lib/one.go\n" +
	"1:1\n" +
	"9:Just this\n"

// hashTest links HASH-1-2 to a test and has an orphaned annotation for
// HASH-9-9, leaving the other hashed items untested. It is built from lines
// so that the annotations aren't read as annotations of this file.
var hashTest = strings.Join([]string{
	"package pkg",
	"",
	`import "testing"`,
	"",
	"// HASH-1-2",
	"func TestTwo(t *testing.T) {}",
	"",
	"// HASH-9-9",
	"func TestOrphan(t *testing.T) {}",
	"",
}, "\n")

// hashRegexp matches stdout holding a hash and nothing else (SPEC-5-1).
var hashRegexp = regexp.MustCompile(`\A[0-9a-f]{64}\n\z`)

// wantHash returns the hex SHA-256 of a hand-written canonical form, after
// checking that each of its lines is "<n>:<field>" with n the field's length
// in bytes, so that a miscounted fixture fails as a fixture.
func wantHash(t *testing.T, canonical string) string {
	t.Helper()
	if !strings.HasSuffix(canonical, "\n") {
		t.Fatalf("fixture: canonical form doesn't end in a newline")
	}
	for _, line := range strings.Split(strings.TrimSuffix(canonical, "\n"), "\n") {
		n, field, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("fixture: line %q has no colon", line)
		}
		if want, err := strconv.Atoi(n); err != nil || want != len(field) {
			t.Fatalf("fixture: line %q: length %s, field is %d bytes", line, n, len(field))
		}
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// hashProject writes a project with the given .spool.yaml and files into a
// temporary directory and returns its root.
func hashProject(t *testing.T, spoolYAML string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{".spool.yaml": spoolYAML}
	for p, c := range files {
		all[p] = c
	}
	writeFiles(t, root, all)
	return root
}

// offConfig scans Go test files with every severity off.
var offConfig = severityConfig(config.SeverityOff, config.SeverityOff, config.SeverityOff)

// runHash calls Hash in root and returns its exit status, stdout and stderr.
func runHash(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Hash(args, root, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// hashOf runs spool hash with ids against the given .req files and returns
// the printed hash, failing unless it exits 0 with a hash on stdout.
func hashOf(t *testing.T, reqs map[string]string, ids string) string {
	t.Helper()
	root := hashProject(t, offConfig, reqs)
	code, stdout, stderr := runHash(t, root, ids)
	if code != 0 || !hashRegexp.MatchString(stdout) {
		t.Fatalf("exit = %d, stdout = %q, want 0 and a hash\nstderr:\n%s", code, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n")
}

// edit returns s with old, which must occur exactly once, replaced by repl.
func edit(t *testing.T, s, old, repl string) string {
	t.Helper()
	if n := strings.Count(s, old); n != 1 {
		t.Fatalf("fixture: %q occurs %d times, want once", old, n)
	}
	return strings.Replace(s, old, repl, 1)
}

// SPEC-5-1
func TestHash_Printed(t *testing.T) {
	want := wantHash(t, hashCanonical)
	for _, tc := range []struct {
		name string
		yaml string
	}{
		{"severities error", severityConfig(config.SeverityError, config.SeverityError, config.SeverityError)},
		{"severities warning", severityConfig(config.SeverityWarning, config.SeverityWarning, config.SeverityWarning)},
		{"severities off", offConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := hashProject(t, tc.yaml, map[string]string{
				".spool/hash.req":  hashReq,
				"pkg/hash_test.go": hashTest,
			})
			// HASH-1-2 is covered; the other listed items are untested.
			for _, ids := range []string{hashIDs, "HASH-1-2", "HASH-2-1,HASH-3-INV-2"} {
				code, stdout, stderr := runHash(t, root, ids)
				if code != 0 {
					t.Errorf("%s: exit = %d, want 0\nstderr:\n%s", ids, code, stderr)
				}
				if !hashRegexp.MatchString(stdout) {
					t.Errorf("%s: stdout = %q, want 64 lowercase hex digits and a newline", ids, stdout)
				}
				if ids == hashIDs && stdout != want+"\n" {
					t.Errorf("%s: stdout = %q, want %q", ids, stdout, want+"\n")
				}
			}
		})
	}
}

// SPEC-5-2
func TestHash_CanonicalForm(t *testing.T) {
	want := wantHash(t, hashCanonical)
	if got := hashOf(t, map[string]string{".spool/hash.req": hashReq}, hashIDs); got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}

// SPEC-5-2
func TestHash_CanonicalFormOneItem(t *testing.T) {
	want := wantHash(t, ""+
		"12:HASH-1-INV-1\n"+
		"3:Inv\n"+
		"13:Always holds.\n"+
		"6:HASH-1\n"+
		"12:Spaced Title\n"+
		"29:A requirement over two lines.\n"+
		"1:2\n"+
		"13:internal/b.go\n"+
		"13:internal/a.go\n"+
		"1:2\n"+
		"12:Not this one\n"+
		"8:Nor that\n")
	if got := hashOf(t, map[string]string{".spool/hash.req": hashReq}, "HASH-1-INV-1"); got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}

// SPEC-5-2
func TestHash_CanonicalFormUnicodeWhitespace(t *testing.T) {
	// No-break, ideographic, line and paragraph separator spaces collapse
	// and are trimmed; zero-width spaces are kept as written.
	req := edit(t, hashReq, "title: Inv\n", "title: \"\\u3000In\\u00a0\\u2028v\\u200b\\u2029\"\n")
	req = edit(t, req, "description: Always holds.\n", "description: \"\\u200b\\u00a0Always\\u3000\\u3000holds.\\u2028\"\n")
	want := wantHash(t, ""+
		"12:HASH-1-INV-1\n"+
		"7:In v\u200b\n"+
		"17:\u200b Always holds.\n"+
		"6:HASH-1\n"+
		"12:Spaced Title\n"+
		"29:A requirement over two lines.\n"+
		"1:2\n"+
		"13:internal/b.go\n"+
		"13:internal/a.go\n"+
		"1:2\n"+
		"12:Not this one\n"+
		"8:Nor that\n")
	if got := hashOf(t, map[string]string{".spool/hash.req": req}, "HASH-1-INV-1"); got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}

// SPEC-5-3
func TestHash_CoveredFieldsChangeTheHash(t *testing.T) {
	base := wantHash(t, hashCanonical)
	for _, tc := range []struct {
		name string
		old  string // replaced in hashReq by new, unless both are empty
		new  string
		ids  string
	}{
		{"item id", "- id: HASH-1-2\n", "- id: HASH-1-5\n", strings.Replace(hashIDs, "HASH-1-2", "HASH-1-5", -1)},
		{"item title", "title: Two\n", "title: Too\n", hashIDs},
		{"item description", "then z.", "then w.", hashIDs},
		{"invariant title", "title: Single Inv\n", "title: Single Invariant\n", hashIDs},
		{"invariant description", "Always holds.", "Always hold.", hashIDs},
		{"requirement id", "- id: HASH-1\n", "- id: HASH-4\n", hashIDs},
		{"requirement title", "Spaced\\t Title", "Spaced\\t Titles", hashIDs},
		{"requirement description", "over two lines.", "over three lines.", hashIDs},
		{"empty requirement description gains text", "title: Bare\n", "title: Bare\n    description: Now described.\n", hashIDs},
		{"interface path changed", "- internal/a.go\n", "- internal/c.go\n", hashIDs},
		{"interface path added", "- internal/a.go\n", "- internal/a.go\n      - internal/c.go\n", hashIDs},
		{"interface path removed", "      - internal/a.go\n", "", hashIDs},
		{"interface added", "title: Bare\n", "title: Bare\n    interface: lib/bare.go\n", hashIDs},
		{"out_of_scope item changed", "- Nor that\n", "- Nor those\n", hashIDs},
		{"out_of_scope item added", "- Nor that\n", "- Nor that\n      - Nor the other\n", hashIDs},
		{"out_of_scope item removed", "      - Nor that\n", "", hashIDs},
		{"out_of_scope added", "title: Bare\n", "title: Bare\n    out_of_scope: Bare scope\n", hashIDs},
		{"interface path moved to out_of_scope", "      - internal/a.go\n    out_of_scope:\n", "    out_of_scope:\n      - internal/a.go\n", hashIDs},
		{"word moved from title to description", "title: Two\n        description: Given x,", "title: Two Given\n        description: x,", hashIDs},
		{"zero-width space in item title", "title: Two\n", "title: T\u200bwo\n", hashIDs},
		{"zero-width space at end of item title", "title: Two\n", "title: Two\u200b\n", hashIDs},
		{"zero-width space beside a space", "then z.", "then \u200bz.", hashIDs},
		{"zero-width space for a space", "then z.", "then\u200bz.", hashIDs},
		{"list gains an item", "", "", hashIDs + ",HASH-1-3"},
		{"list loses an item", "", "", "hash-1-010,HASH-1-INV-1,HASH-3-INV-02,HASH-1-2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := hashReq
			if tc.old != "" || tc.new != "" {
				req = edit(t, req, tc.old, tc.new)
			}
			if got := hashOf(t, map[string]string{".spool/hash.req": req}, tc.ids); got == base {
				t.Errorf("hash = %q, unchanged, want it to change", got)
			}
		})
	}
}

// SPEC-5-4
func TestHash_UnknownIDs(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  string
		want []string
	}{
		{"unknown ac", "HASH-1-2,HASH-7-1", []string{"HASH-7-1"}},
		{"unknown invariant", "HASH-1-INV-9,HASH-1-2", []string{"HASH-1-INV-9"}},
		{"unknown feature", "NOPE-1-1", []string{"NOPE-1-1"}},
		{"orphaned annotation id", "HASH-9-9", []string{"HASH-9-9"}},
		{"several unknown", "HASH-7-1,HASH-1-2,NOPE-2-INV-3,HASH-1-99", []string{"HASH-7-1", "NOPE-2-INV-3", "HASH-1-99"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := hashProject(t, offConfig, map[string]string{
				".spool/hash.req":  hashReq,
				"pkg/hash_test.go": hashTest,
			})
			code, stdout, stderr := runHash(t, root, tc.ids)
			if code != 1 {
				t.Errorf("exit = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			for _, w := range tc.want {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr = %q, want it to name %q", stderr, w)
				}
			}
		})
	}
}

// SPEC-5-5
func TestHash_BadArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"no id list", nil, nil},
		{"two arguments", []string{"HASH-1-2", "HASH-2-1"}, nil},
		{"three arguments", []string{"HASH-1-2,HASH-1-10", "HASH-2-1", "HASH-3-INV-2"}, nil},
		{"empty list", []string{""}, nil},
		{"empty item", []string{"HASH-1-2,,HASH-2-1"}, nil},
		{"leading empty item", []string{",HASH-1-2"}, nil},
		{"trailing empty item", []string{"HASH-1-2,"}, nil},
		{"malformed id", []string{"HASH-1-2,not-an-id"}, []string{"not-an-id"}},
		{"malformed invariant id", []string{"HASH-1-INV,HASH-1-2"}, []string{"HASH-1-INV"}},
		{"malformed id not normalized", []string{"HASH-1-2,hash-01-inv"}, []string{"hash-01-inv"}},
		{"requirement id", []string{"HASH-1-2,SLICE-8"}, []string{"SLICE-8"}},
		{"requirement id not normalized", []string{"HASH-1-2,slice-08"}, []string{"slice-08"}},
		{"defined requirement id", []string{"HASH-1"}, []string{"HASH-1"}},
		{"defined requirement id not normalized", []string{"HASH-2-1,hash-03"}, []string{"hash-03"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := hashProject(t, offConfig, map[string]string{".spool/hash.req": hashReq})
			code, stdout, stderr := runHash(t, root, tc.args...)
			checkUsageError(t, code, stdout, stderr, tc.want...)
		})
	}
}

// SPEC-5-INV-1
func TestHash_IgnoresWhatItDoesNotCover(t *testing.T) {
	want := wantHash(t, hashCanonical)
	type change struct{ old, new string }
	for _, tc := range []struct {
		name    string
		changes []change
		ids     string
		extra   map[string]string // more files, or replacements, by path
	}{
		{name: "ids as listed", ids: hashIDs},
		{name: "ids reordered", ids: "HASH-3-INV-2,HASH-2-1,HASH-1-INV-1,HASH-1-2,HASH-1-10"},
		{name: "ids repeated", ids: "HASH-1-10,HASH-1-10,HASH-1-2,HASH-1-INV-1,HASH-2-1,HASH-3-INV-2,HASH-2-1"},
		{name: "ids lower case", ids: "hash-1-10,hash-1-2,hash-1-inv-1,hash-2-1,hash-3-inv-2"},
		{name: "ids with leading zeros", ids: "HASH-01-10,HASH-1-002,HASH-001-INV-01,HASH-02-01,HASH-3-INV-0002"},
		{name: "whitespace in item title", ids: hashIDs, changes: []change{{"title: Two\n", "title: \"\\tTwo  \"\n"}}},
		{name: "whitespace in item description", ids: hashIDs, changes: []change{{
			"description: Given x, when y, then z.\n",
			"description: >\n          Given x,\n          when y,   then z.\n\n",
		}}},
		{name: "whitespace in literal description", ids: hashIDs, changes: []change{{"Literal\n            block.\n", "Literal block.\n"}}},
		{name: "whitespace in invariant description", ids: hashIDs, changes: []change{{"\"Tab\\there.\"", "\"  Tab \\n\\t here. \""}}},
		{name: "whitespace in requirement title", ids: hashIDs, changes: []change{{"\"  Spaced\\t Title  \"", "Spaced Title"}}},
		{name: "whitespace in requirement description", ids: hashIDs, changes: []change{{
			"      A requirement\n      over two lines.\n",
			"      A   requirement over\n      two\tlines.\n",
		}}},
		{name: "whitespace in out_of_scope item", ids: hashIDs, changes: []change{{"\"Not  this\\tone \"", "Not this one"}, {"- Nor that\n", "- \" Nor\\n that\"\n"}}},
		{name: "unicode whitespace in item title", ids: hashIDs, changes: []change{{"title: Two\n", "title: \"\\u3000Two\\u00a0\"\n"}}},
		{name: "unicode whitespace in item description", ids: hashIDs, changes: []change{{
			"description: Given x, when y, then z.\n",
			"description: \"Given\\u00a0x,\\u2028when y,\\u3000\\u3000then z.\\u2029\"\n",
		}}},
		{name: "unicode whitespace in out_of_scope item", ids: hashIDs, changes: []change{{"\"Not  this\\tone \"", "\"\\u00a0Not\\u2028this\\u3000one\\u0085\""}}},
		{name: "unicode whitespace in plain scalars", ids: hashIDs, changes: []change{{"title: Two\n", "title: Two\u00a0\n"}, {"Always holds.", "Always\u3000holds."}}},
		{name: "status", ids: hashIDs, changes: []change{
			{"rationale: Because.\n", "rationale: Because.\n    status: deprecated\n"},
			{"    status: active\n    deciders:", "    deciders:"},
			{"title: Bare\n    status: draft\n", "title: Bare\n    status: active\n"},
			{"title: Two\n", "title: Two\n        status: draft\n"},
		}},
		{name: "rationale", ids: hashIDs, changes: []change{{"rationale: Because.\n", "rationale: Since it must.\n"}}},
		{name: "deciders", ids: hashIDs, changes: []change{{"      - Ada\n", "      - Grace\n      - Alan\n"}}},
		{name: "consulted", ids: hashIDs, changes: []change{{"consulted: []\n", "consulted:\n      - Barbara\n"}}},
		{name: "date", ids: hashIDs, changes: []change{{"date: 2026-11-01\n", "date: 2027-01-31\n"}}},
		{name: "superseded_by", ids: hashIDs, changes: []change{{"superseded_by:\n", "superseded_by: HASH-3\n"}}},
		{name: "other items", ids: hashIDs, changes: []change{
			{"title: Unlisted\n", "title: Still Unlisted\n"},
			{"then it is left out.", "then it stays out."},
			{"title: Unlisted Inv\n        description: Never listed.\n", "title: Changed Inv\n        description: Changed.\n      - id: HASH-1-INV-3\n        title: New Inv\n        description: New.\n"},
			{"then counts are zero.\n", "then counts are zero.\n      - id: HASH-2-2\n        title: New AC\n        description: Given a new AC, when hashed, then it is left out.\n"},
		}},
		{name: "other requirements", ids: hashIDs, changes: []change{{"  - id: HASH-3\n", "  - id: HASH-4\n    title: Other\n    description: Another requirement.\n    status: active\n    interface: lib/other.go\n    acceptance_criteria:\n      - id: HASH-4-1\n        title: Other AC\n        description: Given another, when hashed, then it is left out.\n\n  - id: HASH-3\n"}},
			extra: map[string]string{".spool/other.req": "feature: OTHER\n\nrequirements:\n  - id: OTHER-1\n    title: Other\n    description: Other.\n    status: active\n    acceptance_criteria:\n      - id: OTHER-1-1\n        title: Other\n        description: Given other, when hashed, then it is left out.\n"},
		},
		{name: "requirement moved to another file", ids: hashIDs, changes: []change{{hashReq[strings.Index(hashReq, "  - id: HASH-3\n"):], ""}},
			extra: map[string]string{".spool/more/hash3.req": "feature: HASH\n\nrequirements:\n\n" + hashReq[strings.Index(hashReq, "  - id: HASH-3\n"):]},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := hashReq
			for _, c := range tc.changes {
				req = edit(t, req, c.old, c.new)
			}
			files := map[string]string{".spool/hash.req": req}
			for p, c := range tc.extra {
				files[p] = c
			}
			if got := hashOf(t, files, tc.ids); got != want {
				t.Errorf("hash = %q, want %q", got, want)
			}
		})
	}
}
