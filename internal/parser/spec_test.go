package parser

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// specReqYAML returns a .req file with a valid requirement DEMO-1 followed by
// the requirement DEMO-2, whose body holds the given field lines.
func specReqYAML(fieldLines string) string {
	return `feature: DEMO

requirements:
  - id: DEMO-1
    title: First
    description: A requirement.
    status: active
    acceptance_criteria:
      - id: DEMO-1-1
        title: One
        description: Given a, when b, then c.
  - id: DEMO-2
    title: Second
    description: Another requirement.
    status: active
` + fieldLines + `    acceptance_criteria:
      - id: DEMO-2-1
        title: Two
        description: Given a, when b, then c.
`
}

// parseSpecReq writes specReqYAML(fieldLines) to a temporary file and parses
// it.
func parseSpecReq(t *testing.T, fieldLines string) (*ReqFile, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "demo.req")
	writeReqFile(t, path, specReqYAML(fieldLines))
	return ParseFile(path)
}

// SPEC-1-1
func TestParseFile_Interface(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
		want   []string
	}{
		{"single path", "    interface: internal/lint/lint.go\n", []string{"internal/lint/lint.go"}},
		{"single path quoted", "    interface: \"./spool.go\"\n", []string{"./spool.go"}},
		{"list", "    interface:\n      - spool.go\n      - internal/lint/lint.go\n      - ./internal/lsp/\n",
			[]string{"spool.go", "internal/lint/lint.go", "./internal/lsp/"}},
		{"flow list", "    interface: [b.go, a.go]\n", []string{"b.go", "a.go"}},
		{"list of one", "    interface:\n      - only.go\n", []string{"only.go"}},
		{"none", "", nil},
		{"empty list", "    interface: []\n", nil},
		{"null", "    interface:\n", nil},
		{"null written", "    interface: null\n", nil},
		{"tilde", "    interface: ~\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rf, err := parseSpecReq(t, tc.fields)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if len(rf.Requirements) != 2 {
				t.Fatalf("len(Requirements) = %d, want 2", len(rf.Requirements))
			}
			if got := rf.Requirements[0].Interface; len(got) != 0 {
				t.Errorf("DEMO-1 Interface = %q, want none", got)
			}
			got := []string(rf.Requirements[1].Interface)
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Errorf("Interface = %q, want none", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Interface = %q, want %q", got, tc.want)
			}
		})
	}
}

// SPEC-1-2
func TestParseFile_MalformedInterface(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
	}{
		{"map", "    interface:\n      path: spool.go\n"},
		{"flow map", "    interface: {path: spool.go}\n"},
		{"list holding a map", "    interface:\n      - spool.go\n      - path: lint.go\n"},
		{"list holding a list", "    interface:\n      - spool.go\n      - [a.go, b.go]\n"},
		{"empty path", "    interface: \"\"\n"},
		{"list holding an empty path", "    interface:\n      - spool.go\n      - \"\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rf, err := parseSpecReq(t, tc.fields)
			if err == nil {
				t.Fatalf("ParseFile succeeded with Interface %q, want an error", rf.Requirements[1].Interface)
			}
			msg := err.Error()
			if !strings.Contains(msg, "DEMO-2") {
				t.Errorf("error %q does not name the requirement DEMO-2", msg)
			}
			if !strings.Contains(msg, "interface") {
				t.Errorf("error %q does not name the interface field", msg)
			}
		})
	}
}

// SPEC-2-1
func TestParseFile_OutOfScope(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
		want   []string
	}{
		{"single item", "    out_of_scope: Globs in paths\n", []string{"Globs in paths"}},
		{"list", "    out_of_scope:\n      - Globs in paths\n      - Adding it to the --ids output (COVERAGE-2-6 fixes its keys)\n      - Checking what the stub contains\n",
			[]string{"Globs in paths", "Adding it to the --ids output (COVERAGE-2-6 fixes its keys)", "Checking what the stub contains"}},
		{"list of one", "    out_of_scope:\n      - Only this\n", []string{"Only this"}},
		{"none", "", nil},
		{"empty list", "    out_of_scope: []\n", nil},
		{"null", "    out_of_scope:\n", nil},
		{"null written", "    out_of_scope: null\n", nil},
		{"tilde", "    out_of_scope: ~\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rf, err := parseSpecReq(t, tc.fields)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if len(rf.Requirements) != 2 {
				t.Fatalf("len(Requirements) = %d, want 2", len(rf.Requirements))
			}
			if got := rf.Requirements[0].OutOfScope; len(got) != 0 {
				t.Errorf("DEMO-1 OutOfScope = %q, want none", got)
			}
			got := []string(rf.Requirements[1].OutOfScope)
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Errorf("OutOfScope = %q, want none", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("OutOfScope = %q, want %q", got, tc.want)
			}
		})
	}
}

// SPEC-2-1
func TestParseFile_MalformedOutOfScope(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
	}{
		{"map", "    out_of_scope:\n      item: Globs\n"},
		{"flow map", "    out_of_scope: {item: Globs}\n"},
		{"list holding a map", "    out_of_scope:\n      - Globs\n      - item: Stubs\n"},
		{"list holding a list", "    out_of_scope:\n      - Globs\n      - [Stubs, Paths]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rf, err := parseSpecReq(t, tc.fields)
			if err == nil {
				t.Fatalf("ParseFile succeeded with OutOfScope %q, want an error", rf.Requirements[1].OutOfScope)
			}
			msg := err.Error()
			if !strings.Contains(msg, "DEMO-2") {
				t.Errorf("error %q does not name the requirement DEMO-2", msg)
			}
			if !strings.Contains(msg, "out_of_scope") {
				t.Errorf("error %q does not name the out_of_scope field", msg)
			}
		})
	}
}
