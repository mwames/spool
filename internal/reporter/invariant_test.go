package reporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

func inv(id, desc string) parser.Invariant {
	return parser.Invariant{ID: id, Title: id, Description: desc}
}

// withInvariants returns r with the given invariants, as parsed: one with no
// status of its own takes the requirement's (SPEC-4-1).
func withInvariants(r parser.Requirement, invs ...parser.Invariant) parser.Requirement {
	for i := range invs {
		if invs[i].Status == "" {
			invs[i].Status = r.Status
		}
	}
	r.Invariants = invs
	return r
}

// invariantCountsReport has invariant counts that differ from every AC count
// and from each other, so each one can be found in formatted output.
func invariantCountsReport() *spool.Report {
	return &spool.Report{
		TotalRequirements:  2,
		TotalACs:           2,
		LinkedACs:          2,
		TotalInvariants:    73,
		LinkedInvariants:   50,
		UntestedInvariants: 23,
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 1, ACs: 1, LinkedACs: 1, Invariants: 31, LinkedInvariants: 20, UntestedInvariants: 11},
			{Name: "CONFIG", Requirements: 1, ACs: 1, LinkedACs: 1, Invariants: 42, LinkedInvariants: 30, UntestedInvariants: 12},
		},
	}
}

// tokens splits s into runs of letters and digits.
func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		out[f] = true
	}
	return out
}

// checkInvariantCounts asserts that a line naming each feature carries its
// invariant counts, and that the totals appear in the output.
func checkInvariantCounts(t *testing.T, out string) {
	t.Helper()
	features := map[string][]string{
		"AUTH":   {"31", "20", "11"},
		"CONFIG": {"42", "30", "12"},
	}
	for name, want := range features {
		found := false
		for _, line := range strings.Split(out, "\n") {
			toks := tokens(line)
			if !toks[name] {
				continue
			}
			all := true
			for _, w := range want {
				all = all && toks[w]
			}
			if all {
				found = true
			}
		}
		if !found {
			t.Errorf("no line for %s carries invariant counts %v:\n%s", name, want, out)
		}
	}
	toks := tokens(out)
	for _, w := range []string{"73", "50", "23"} {
		if !toks[w] {
			t.Errorf("output does not carry invariant total %s:\n%s", w, out)
		}
	}
}

// INVARIANT-1-5: Untested Invariants Reported
func TestGenerate_InvariantCounts(t *testing.T) {
	idx := buildIndex(
		[]*parser.ReqFile{
			reqFile("AUTH",
				withInvariants(req("AUTH-1", "active", ac("AUTH-1-1", "login")),
					inv("AUTH-1-INV-1", "sessions expire"),
					inv("AUTH-1-INV-2", "passwords are never logged"),
				),
				withInvariants(req("AUTH-2", "superseded", ac("AUTH-2-1", "old")),
					inv("AUTH-2-INV-1", "old invariant"),
				),
			),
			reqFile("CONFIG",
				withInvariants(req("CONFIG-1", "active"), inv("CONFIG-1-INV-1", "config is never written")),
			),
		},
		[]scanner.FileResult{
			scanResult("auth_test.go", mapping("TestLogin", 5, "AUTH-1-1", "AUTH-1-INV-1")),
			scanResult("config_test.go", mapping("TestConfig", 5, "CONFIG-1-INV-1")),
		},
	)

	r := Generate(idx)

	if r.TotalInvariants != 3 {
		t.Errorf("TotalInvariants = %d, want 3", r.TotalInvariants)
	}
	if r.LinkedInvariants != 2 {
		t.Errorf("LinkedInvariants = %d, want 2", r.LinkedInvariants)
	}
	if r.UntestedInvariants != 1 {
		t.Errorf("UntestedInvariants = %d, want 1", r.UntestedInvariants)
	}

	// ACs are counted separately from invariants.
	if r.TotalACs != 1 || r.LinkedACs != 1 || r.UntestedACs != 0 || len(r.Untested) != 0 {
		t.Errorf("AC counts = %d total, %d linked, %d untested, list %v; want 1, 1, 0, empty",
			r.TotalACs, r.LinkedACs, r.UntestedACs, r.Untested)
	}
	if r.OrphanedCount != 0 {
		t.Errorf("OrphanedCount = %d, want 0", r.OrphanedCount)
	}

	if len(r.Features) != 2 {
		t.Fatalf("len(Features) = %d, want 2", len(r.Features))
	}
	want := map[string][3]int{"AUTH": {2, 1, 1}, "CONFIG": {1, 1, 0}}
	for _, f := range r.Features {
		w, ok := want[f.Name]
		if !ok {
			t.Errorf("unexpected feature %q", f.Name)
			continue
		}
		if got := [3]int{f.Invariants, f.LinkedInvariants, f.UntestedInvariants}; got != w {
			t.Errorf("%s invariants/linked/untested = %v, want %v", f.Name, got, w)
		}
		if f.Name == "AUTH" && (f.ACs != 1 || f.LinkedACs != 1 || f.UntestedACs != 0) {
			t.Errorf("AUTH ACs/linked/untested = %d/%d/%d, want 1/1/0", f.ACs, f.LinkedACs, f.UntestedACs)
		}
	}

	wantList := []spool.UntestedInvariant{
		{ID: "AUTH-1-INV-2", Description: "passwords are never logged", Feature: "AUTH"},
	}
	if len(r.UntestedInvariantList) != len(wantList) || r.UntestedInvariantList[0] != wantList[0] {
		t.Errorf("UntestedInvariantList = %+v, want %+v", r.UntestedInvariantList, wantList)
	}
}

// INVARIANT-1-5: Untested Invariants Reported
func TestTextFormatter_UntestedInvariantsListed(t *testing.T) {
	r := &spool.Report{
		UntestedInvariants: 1,
		UntestedInvariantList: []spool.UntestedInvariant{
			{ID: "AUTH-1-INV-2", Description: "passwords are never logged", Feature: "AUTH"},
		},
	}

	data, err := Text{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	heading := strings.Index(out, "Untested Invariants")
	if heading < 0 {
		t.Fatalf("output should contain an Untested Invariants heading:\n%s", out)
	}
	if i := strings.Index(out, "AUTH-1-INV-2"); i < heading {
		t.Errorf("output should list AUTH-1-INV-2 under the heading:\n%s", out)
	}
	if i := strings.Index(out, "passwords are never logged"); i < heading {
		t.Errorf("output should list the description under the heading:\n%s", out)
	}
}

// INVARIANT-1-5: Untested Invariants Reported
func TestTextFormatter_InvariantCounts(t *testing.T) {
	data, err := Text{}.Format(invariantCountsReport())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkInvariantCounts(t, string(data))
}

// INVARIANT-1-5: Untested Invariants Reported
func TestTextFormatter_FullTraceabilityWithInvariants(t *testing.T) {
	base := spool.Report{
		TotalRequirements: 1,
		TotalACs:          1,
		LinkedACs:         1,
		TotalInvariants:   1,
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 1, ACs: 1, LinkedACs: 1, Invariants: 1},
		},
	}

	untested := base
	untested.UntestedInvariants = 1
	untested.Features = []spool.FeatureReport{{Name: "AUTH", Requirements: 1, ACs: 1, LinkedACs: 1, Invariants: 1, UntestedInvariants: 1}}
	untested.UntestedInvariantList = []spool.UntestedInvariant{
		{ID: "AUTH-1-INV-1", Description: "sessions expire", Feature: "AUTH"},
	}
	data, err := Text{}.Format(&untested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.ToLower(string(data)), "full traceability") {
		t.Errorf("output with an untested invariant should not claim full traceability:\n%s", data)
	}

	linked := base
	linked.LinkedInvariants = 1
	linked.Features = []spool.FeatureReport{{Name: "AUTH", Requirements: 1, ACs: 1, LinkedACs: 1, Invariants: 1, LinkedInvariants: 1}}
	data, err = Text{}.Format(&linked)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "Full traceability") {
		t.Errorf("output with every AC and invariant linked should say Full traceability:\n%s", data)
	}
}

// INVARIANT-1-6: Untested Invariant Severity
func TestExitCode_UntestedInvariantError(t *testing.T) {
	r := &spool.Report{UntestedInvariants: 1, UntestedInvariantList: []spool.UntestedInvariant{{ID: "AUTH-1-INV-1"}}}
	sev := config.SeverityConfig{UntestedAC: config.SeverityOff, OrphanedAnnotation: config.SeverityOff, UntestedInvariant: config.SeverityError}
	if got := ExitCode(r, sev); got != 1 {
		t.Errorf("ExitCode = %d, want 1", got)
	}
}

// INVARIANT-1-6: Untested Invariant Severity
func TestExitCode_UntestedInvariantLowerSeverities(t *testing.T) {
	for _, s := range []config.Severity{config.SeverityWarning, config.SeverityInfo, config.SeverityOff} {
		only := &spool.Report{UntestedInvariants: 1, UntestedInvariantList: []spool.UntestedInvariant{{ID: "AUTH-1-INV-1"}}}
		sev := config.SeverityConfig{UntestedAC: config.SeverityError, OrphanedAnnotation: config.SeverityError, UntestedInvariant: s}
		if got := ExitCode(only, sev); got != 0 {
			t.Errorf("severity %s, only an untested invariant: ExitCode = %d, want 0", s, got)
		}

		withAC := &spool.Report{UntestedACs: 1, UntestedInvariants: 1, UntestedInvariantList: only.UntestedInvariantList}
		if got := ExitCode(withAC, sev); got != 1 {
			t.Errorf("severity %s, untested AC at error: ExitCode = %d, want 1", s, got)
		}
		sev.UntestedAC = config.SeverityWarning
		if got := ExitCode(withAC, sev); got != 0 {
			t.Errorf("severity %s, untested AC at warning: ExitCode = %d, want 0", s, got)
		}
	}
}

// INVARIANT-1-6: Untested Invariant Severity
func TestExitCode_UntestedInvariantDefaultFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".spool.yaml"), []byte("test_patterns:\n  - \"**/*_test.go\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	r := &spool.Report{TotalInvariants: 1, UntestedInvariants: 1, UntestedInvariantList: []spool.UntestedInvariant{{ID: "AUTH-1-INV-1"}}}
	if got := ExitCode(r, cfg.Severity); got != 1 {
		t.Errorf("ExitCode = %d, want 1", got)
	}
}

// INVARIANT-1-7: JSON and Markdown Reports
func TestJSONFormatter_Invariants(t *testing.T) {
	r := invariantCountsReport()
	r.UntestedInvariantList = []spool.UntestedInvariant{
		{ID: "AUTH-1-INV-2", Description: "passwords are never logged", Feature: "AUTH"},
	}

	data, err := JSON{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	for key, want := range map[string]float64{"totalInvariants": 73, "linkedInvariants": 50, "untestedInvariants": 23} {
		if parsed[key] != want {
			t.Errorf("%s = %v, want %v", key, parsed[key], want)
		}
	}

	features, _ := parsed["features"].([]any)
	if len(features) != 2 {
		t.Fatalf("features = %v, want 2 entries", parsed["features"])
	}
	wantFeatures := []map[string]float64{
		{"invariants": 31, "linkedInvariants": 20, "untestedInvariants": 11},
		{"invariants": 42, "linkedInvariants": 30, "untestedInvariants": 12},
	}
	for i, wf := range wantFeatures {
		feat, _ := features[i].(map[string]any)
		for key, want := range wf {
			if feat[key] != want {
				t.Errorf("features[%d].%s = %v, want %v", i, key, feat[key], want)
			}
		}
	}

	list, _ := parsed["untestedInvariantList"].([]any)
	if len(list) != 1 {
		t.Fatalf("untestedInvariantList = %v, want 1 entry", parsed["untestedInvariantList"])
	}
	item, _ := list[0].(map[string]any)
	for key, want := range map[string]string{"id": "AUTH-1-INV-2", "description": "passwords are never logged", "feature": "AUTH"} {
		if item[key] != want {
			t.Errorf("untestedInvariantList[0].%s = %v, want %q", key, item[key], want)
		}
	}
}

// INVARIANT-1-7: JSON and Markdown Reports
func TestJSONFormatter_ExistingFieldsUnchanged(t *testing.T) {
	r := &spool.Report{
		TotalRequirements: 2,
		TotalACs:          3,
		LinkedACs:         1,
		UntestedACs:       2,
		OrphanedCount:     1,
		Features: []spool.FeatureReport{
			{Name: "AUTH", Requirements: 2, ACs: 3, LinkedACs: 1, UntestedACs: 2, ExcludedRequirements: 4,
				Invariants: 5, LinkedInvariants: 2, UntestedInvariants: 3},
		},
		Untested: []spool.UntestedAC{
			{ID: "AUTH-1-1", Description: "login", Feature: "AUTH"},
		},
		Orphaned: []spool.OrphanedRef{
			{ID: "AUTH-99-1", File: "test.go", Line: 5},
		},
		TotalInvariants:    5,
		LinkedInvariants:   2,
		UntestedInvariants: 3,
		UntestedInvariantList: []spool.UntestedInvariant{
			{ID: "AUTH-1-INV-1", Description: "sessions expire", Feature: "AUTH"},
		},
	}

	data, err := JSON{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	for key, want := range map[string]float64{"totalRequirements": 2, "totalACs": 3, "linkedACs": 1, "untestedACs": 2, "orphanedCount": 1} {
		if parsed[key] != want {
			t.Errorf("%s = %v, want %v", key, parsed[key], want)
		}
	}

	features, _ := parsed["features"].([]any)
	if len(features) != 1 {
		t.Fatalf("features = %v, want 1 entry", parsed["features"])
	}
	feat, _ := features[0].(map[string]any)
	if feat["name"] != "AUTH" {
		t.Errorf("features[0].name = %v, want AUTH", feat["name"])
	}
	for key, want := range map[string]float64{"requirements": 2, "acs": 3, "linkedACs": 1, "untestedACs": 2, "excludedRequirements": 4} {
		if feat[key] != want {
			t.Errorf("features[0].%s = %v, want %v", key, feat[key], want)
		}
	}

	untested, _ := parsed["untested"].([]any)
	if len(untested) != 1 {
		t.Fatalf("untested = %v, want 1 entry", parsed["untested"])
	}
	u, _ := untested[0].(map[string]any)
	for key, want := range map[string]string{"id": "AUTH-1-1", "description": "login", "feature": "AUTH"} {
		if u[key] != want {
			t.Errorf("untested[0].%s = %v, want %q", key, u[key], want)
		}
	}

	orphaned, _ := parsed["orphaned"].([]any)
	if len(orphaned) != 1 {
		t.Fatalf("orphaned = %v, want 1 entry", parsed["orphaned"])
	}
	o, _ := orphaned[0].(map[string]any)
	if o["id"] != "AUTH-99-1" || o["file"] != "test.go" || o["line"] != float64(5) {
		t.Errorf("orphaned[0] = %v, want id AUTH-99-1, file test.go, line 5", o)
	}
}

// INVARIANT-1-7: JSON and Markdown Reports
func TestMarkdownFormatter_InvariantCounts(t *testing.T) {
	data, err := Markdown{}.Format(invariantCountsReport())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkInvariantCounts(t, string(data))
}

// INVARIANT-1-7: JSON and Markdown Reports
func TestMarkdownFormatter_UntestedInvariants(t *testing.T) {
	r := &spool.Report{
		UntestedInvariants: 1,
		UntestedInvariantList: []spool.UntestedInvariant{
			{ID: "BILLING-3-INV-2", Description: "charges are never duplicated", Feature: "BILLING"},
		},
	}

	data, err := Markdown{}.Format(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	for _, want := range []string{"BILLING-3-INV-2", "charges are never duplicated"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q:\n%s", want, out)
		}
	}
	// The feature name must appear in its own right, not only as the ID's prefix.
	if !tokens(strings.ReplaceAll(out, "BILLING-3-INV-2", ""))["BILLING"] {
		t.Errorf("output should name the invariant's feature BILLING:\n%s", out)
	}
}
