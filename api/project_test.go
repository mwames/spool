package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/api"
	"github.com/mwames/Spool/internal/reporter"
)

// ----- fixture builder -----

type acDef struct {
	ID, Title, Description string
}

type reqDef struct {
	ID          string
	Title       string
	Description string
	Status      string
	ACs         []acDef
}

type featureDef struct {
	Name string
	Reqs []reqDef
}

type testFileDef struct {
	Path string
	Body string
}

type fixture struct {
	Features      []featureDef
	Tests         []testFileDef
	Config        string
	SkipConfig    bool
	InvalidConfig bool
}

const defaultSpoolYAML = `test_patterns:
  - "**/*_test.*"
`

func (f fixture) build(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	if !f.SkipConfig {
		var contents string
		switch {
		case f.InvalidConfig:
			contents = "test_patterns: ::: not yaml :::\n  -\nbroken"
		case f.Config != "":
			contents = f.Config
		default:
			contents = defaultSpoolYAML
		}
		if err := os.WriteFile(filepath.Join(root, ".spool.yaml"), []byte(contents), 0o644); err != nil {
			t.Fatalf("write .spool.yaml: %v", err)
		}
	}

	if len(f.Features) > 0 {
		reqsDir := filepath.Join(root, ".spool")
		if err := os.MkdirAll(reqsDir, 0o755); err != nil {
			t.Fatalf("mkdir .spool: %v", err)
		}
		for _, feat := range f.Features {
			body := renderReqFile(feat)
			path := filepath.Join(reqsDir, strings.ToLower(feat.Name)+".req")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
		}
	}

	for _, tf := range f.Tests {
		full := filepath.Join(root, tf.Path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", tf.Path, err)
		}
		if err := os.WriteFile(full, []byte(tf.Body), 0o644); err != nil {
			t.Fatalf("write %s: %v", tf.Path, err)
		}
	}

	return root
}

func renderReqFile(feat featureDef) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "feature: %s\n\nrequirements:\n", feat.Name)
	for _, r := range feat.Reqs {
		status := r.Status
		if status == "" {
			status = "active"
		}
		title := r.Title
		if title == "" {
			title = r.ID
		}
		desc := r.Description
		if desc == "" {
			desc = "Requirement description."
		}
		fmt.Fprintf(&sb, "  - id: %s\n", r.ID)
		fmt.Fprintf(&sb, "    title: %s\n", title)
		fmt.Fprintf(&sb, "    description: %s\n", desc)
		fmt.Fprintf(&sb, "    status: %s\n", status)
		fmt.Fprintf(&sb, "    acceptance_criteria:\n")
		for _, ac := range r.ACs {
			acTitle := ac.Title
			if acTitle == "" {
				acTitle = ac.ID
			}
			acDesc := ac.Description
			if acDesc == "" {
				acDesc = "AC description."
			}
			fmt.Fprintf(&sb, "      - id: %s\n", ac.ID)
			fmt.Fprintf(&sb, "        title: %s\n", acTitle)
			fmt.Fprintf(&sb, "        description: %s\n", acDesc)
		}
	}
	return sb.String()
}

func goTest(funcName string, annotationIDs ...string) string {
	var sb strings.Builder
	sb.WriteString("package fixture\n\nimport \"testing\"\n\n")
	for _, id := range annotationIDs {
		fmt.Fprintf(&sb, "// %s\n", id)
	}
	fmt.Fprintf(&sb, "func %s(t *testing.T) {}\n", funcName)
	return sb.String()
}

func writeReq(t *testing.T, root, filename string, feat featureDef) {
	t.Helper()
	path := filepath.Join(root, ".spool", filename)
	if err := os.WriteFile(path, []byte(renderReqFile(feat)), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustLoad(t *testing.T, root string) *api.Project {
	t.Helper()
	proj, err := api.LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject(%s): %v", root, err)
	}
	return proj
}

// ----- API-1: Project Loading Contract -----

// API-1-1
func TestLoadProject_RelativePathRejected(t *testing.T) {
	_, err := api.LoadProject("./relative/path")
	var pe *api.ProjectError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want *ProjectError", err)
	}
	if pe.Kind != api.ErrInvalidProject {
		t.Errorf("kind = %q, want %q", pe.Kind, api.ErrInvalidProject)
	}
}

// API-1-2
func TestLoadProject_NonExistentPathRejected(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := api.LoadProject(root)
	var pe *api.ProjectError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want *ProjectError", err)
	}
	if pe.Kind != api.ErrInvalidProject {
		t.Errorf("kind = %q, want %q", pe.Kind, api.ErrInvalidProject)
	}
}

// API-1-3
func TestLoadProject_MissingConfigDetected(t *testing.T) {
	root := fixture{SkipConfig: true}.build(t)
	_, err := api.LoadProject(root)
	var pe *api.ProjectError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want *ProjectError", err)
	}
	if pe.Kind != api.ErrMissingConfig {
		t.Errorf("kind = %q, want %q", pe.Kind, api.ErrMissingConfig)
	}
}

// API-1-4
func TestLoadProject_InvalidConfigDetected(t *testing.T) {
	root := fixture{InvalidConfig: true}.build(t)
	_, err := api.LoadProject(root)
	var pe *api.ProjectError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want *ProjectError", err)
	}
	if pe.Kind != api.ErrInvalidConfig {
		t.Errorf("kind = %q, want %q", pe.Kind, api.ErrInvalidConfig)
	}
}

// ----- API-2: Partial-State Tolerance -----

// API-2-1
func TestLoadProject_MissingReqsDirLoadsEmpty(t *testing.T) {
	// fixture{} writes only .spool.yaml — no .spool/ directory.
	root := fixture{}.build(t)
	proj, err := api.LoadProject(root)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if proj == nil {
		t.Fatalf("expected non-nil project")
	}
	if len(proj.Features()) != 0 {
		t.Errorf("Features = %+v, want empty", proj.Features())
	}
	if got := proj.Requirements(api.RequirementFilter{}); len(got) != 0 {
		t.Errorf("Requirements = %+v, want empty", got)
	}
}

// API-2-2
func TestLoadProject_MalformedReqFileSurfacesParseWarning(t *testing.T) {
	root := fixture{
		Features: []featureDef{{Name: "AUTH", Reqs: []reqDef{
			{ID: "AUTH-1", ACs: []acDef{{ID: "AUTH-1-1"}}},
		}}},
	}.build(t)
	bad := filepath.Join(root, ".spool", "broken.req")
	if err := os.WriteFile(bad, []byte("not: [valid yaml :::\n"), 0o644); err != nil {
		t.Fatalf("write broken: %v", err)
	}

	proj := mustLoad(t, root)

	// Valid file's requirement is still present.
	if proj.Requirement("AUTH-1") == nil {
		t.Errorf("AUTH-1 missing — expected valid file to still parse")
	}

	// Warning identifies the malformed file.
	found := false
	for _, w := range proj.Warnings() {
		if w.Kind == api.WarningParse && strings.Contains(w.Message, "broken.req") {
			found = true
		}
	}
	if !found {
		t.Errorf("no parse warning identifying broken.req in %+v", proj.Warnings())
	}
}

// API-2-3
func TestLoadProject_UnhandledTestFileSurfacesScanWarning(t *testing.T) {
	root := fixture{
		Features: []featureDef{{Name: "AUTH", Reqs: []reqDef{
			{ID: "AUTH-1", ACs: []acDef{{ID: "AUTH-1-1"}}},
		}}},
		Tests: []testFileDef{
			{Path: "weird_test.unknown", Body: "no interpreter handles this"},
		},
	}.build(t)

	proj := mustLoad(t, root)

	found := false
	for _, w := range proj.Warnings() {
		if w.Kind == api.WarningScan && strings.Contains(w.Message, "weird_test.unknown") {
			found = true
		}
	}
	if !found {
		t.Errorf("no scan warning identifying weird_test.unknown in %+v", proj.Warnings())
	}
}

// ----- API-3: Project Query Surface -----

func querySurfaceFixture(t *testing.T) string {
	t.Helper()
	return fixture{
		Features: []featureDef{
			{Name: "AUTH", Reqs: []reqDef{
				{ID: "AUTH-1", Title: "Login", Status: "active", ACs: []acDef{
					{ID: "AUTH-1-1", Title: "Valid creds"},
					{ID: "AUTH-1-2", Title: "Bad creds"},
				}},
				{ID: "AUTH-2", Title: "Logout", Status: "superseded", ACs: []acDef{
					{ID: "AUTH-2-1", Title: "Old"},
				}},
			}},
			{Name: "BILL", Reqs: []reqDef{
				{ID: "BILL-1", Title: "Invoices", Status: "active", ACs: []acDef{
					{ID: "BILL-1-1", Title: "Generated"},
				}},
			}},
		},
		Tests: []testFileDef{
			{Path: "auth_test.go", Body: goTest("TestLogin", "AUTH-1-1")},
		},
	}.build(t)
}

// API-3-1
func TestProject_RequirementLookupReturnsRequirementWithACs(t *testing.T) {
	proj := mustLoad(t, querySurfaceFixture(t))
	view := proj.Requirement("AUTH-1")
	if view == nil {
		t.Fatalf("Requirement(AUTH-1) is nil")
	}
	if view.ID != "AUTH-1" || view.Title != "Login" || view.Status != "active" || view.Feature != "AUTH" {
		t.Errorf("scalars wrong: %+v", view)
	}
	if len(view.AcceptanceCriteria) != 2 {
		t.Fatalf("ACs len = %d, want 2", len(view.AcceptanceCriteria))
	}
	var ac1 *api.AcceptanceCriterionView
	for i := range view.AcceptanceCriteria {
		if view.AcceptanceCriteria[i].ID == "AUTH-1-1" {
			ac1 = &view.AcceptanceCriteria[i]
		}
	}
	if ac1 == nil {
		t.Fatalf("AUTH-1-1 missing from ACs")
	}
	if len(ac1.Tests) != 1 || ac1.Tests[0].Function != "TestLogin" {
		t.Errorf("tests on AUTH-1-1 wrong: %+v", ac1.Tests)
	}
	if !strings.HasSuffix(ac1.Tests[0].File, "auth_test.go") {
		t.Errorf("test file = %q, want ending in auth_test.go", ac1.Tests[0].File)
	}
	if ac1.Tests[0].Line == 0 {
		t.Errorf("test line is 0")
	}
}

// API-3-2
func TestProject_RequirementLookupNilForNonRequirementID(t *testing.T) {
	proj := mustLoad(t, querySurfaceFixture(t))
	if v := proj.Requirement("AUTH-1-1"); v != nil {
		t.Errorf("Requirement(AC id) = %+v, want nil", v)
	}
	if v := proj.Requirement("UNKNOWN-9"); v != nil {
		t.Errorf("Requirement(unknown) = %+v, want nil", v)
	}
}

// API-3-3
func TestProject_ACLookupReturnsACWithParentTitle(t *testing.T) {
	proj := mustLoad(t, querySurfaceFixture(t))
	view := proj.AC("AUTH-1-1")
	if view == nil {
		t.Fatalf("AC(AUTH-1-1) is nil")
	}
	if view.ID != "AUTH-1-1" || view.Title != "Valid creds" || view.Feature != "AUTH" {
		t.Errorf("scalars wrong: %+v", view)
	}
	if view.RequirementID != "AUTH-1" || view.RequirementTitle != "Login" {
		t.Errorf("parent = (%q, %q), want (AUTH-1, Login)", view.RequirementID, view.RequirementTitle)
	}
	if len(view.Tests) != 1 || view.Tests[0].Function != "TestLogin" {
		t.Errorf("tests wrong: %+v", view.Tests)
	}
}

// API-3-4
func TestProject_ACLookupNilForNonACID(t *testing.T) {
	proj := mustLoad(t, querySurfaceFixture(t))
	if v := proj.AC("AUTH-1"); v != nil {
		t.Errorf("AC(req id) = %+v, want nil", v)
	}
	if v := proj.AC("UNKNOWN-9-9"); v != nil {
		t.Errorf("AC(unknown) = %+v, want nil", v)
	}
}

// API-3-5
func TestProject_RequirementsListsSortedWithoutInlinedACs(t *testing.T) {
	proj := mustLoad(t, querySurfaceFixture(t))
	got := proj.Requirements(api.RequirementFilter{})
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (AUTH-1, AUTH-2, BILL-1)", len(got))
	}
	ids := make([]string, len(got))
	for i, r := range got {
		ids[i] = r.ID
		if len(r.AcceptanceCriteria) != 0 {
			t.Errorf("entry %s has %d inlined ACs, want 0", r.ID, len(r.AcceptanceCriteria))
		}
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	for i := range ids {
		if ids[i] != sorted[i] {
			t.Errorf("ordering = %v, want sorted %v", ids, sorted)
			return
		}
	}
}

// API-3-6
func TestProject_RequirementsAppliesFilters(t *testing.T) {
	proj := mustLoad(t, querySurfaceFixture(t))

	byFeature := proj.Requirements(api.RequirementFilter{Feature: "BILL"})
	if len(byFeature) != 1 || byFeature[0].ID != "BILL-1" {
		t.Errorf("Feature filter = %+v, want only BILL-1", byFeature)
	}

	byStatus := proj.Requirements(api.RequirementFilter{Status: "superseded"})
	if len(byStatus) != 1 || byStatus[0].ID != "AUTH-2" {
		t.Errorf("Status filter = %+v, want only AUTH-2", byStatus)
	}

	combined := proj.Requirements(api.RequirementFilter{Feature: "AUTH", Status: "active"})
	if len(combined) != 1 || combined[0].ID != "AUTH-1" {
		t.Errorf("Combined filter = %+v, want only AUTH-1", combined)
	}
}

// API-3-7
func TestProject_FeaturesListsSortedWithCounts(t *testing.T) {
	proj := mustLoad(t, querySurfaceFixture(t))
	feats := proj.Features()
	if len(feats) != 2 {
		t.Fatalf("len = %d, want 2", len(feats))
	}
	if feats[0].Name != "AUTH" || feats[1].Name != "BILL" {
		t.Errorf("ordering = [%q, %q], want [AUTH, BILL]", feats[0].Name, feats[1].Name)
	}
	if feats[0].RequirementCount != 2 || feats[0].ACCount != 3 {
		t.Errorf("AUTH counts = (%d, %d), want (2, 3)", feats[0].RequirementCount, feats[0].ACCount)
	}
	if feats[1].RequirementCount != 1 || feats[1].ACCount != 1 {
		t.Errorf("BILL counts = (%d, %d), want (1, 1)", feats[1].RequirementCount, feats[1].ACCount)
	}
}

// API-3-8
func TestProject_ValidationErrorsSurfaceIndexErrors(t *testing.T) {
	t.Run("duplicate requirement id", func(t *testing.T) {
		root := fixture{
			Features: []featureDef{{Name: "AUTH", Reqs: []reqDef{
				{ID: "AUTH-1", ACs: []acDef{{ID: "AUTH-1-1"}}},
			}}},
		}.build(t)
		writeReq(t, root, "auth_dup.req", featureDef{Name: "AUTH", Reqs: []reqDef{
			{ID: "AUTH-1", ACs: []acDef{{ID: "AUTH-1-2"}}},
		}})
		proj := mustLoad(t, root)
		if !anyError(proj.ValidationErrors(), "duplicate", "AUTH-1") {
			t.Errorf("expected duplicate AUTH-1 error in %+v", proj.ValidationErrors())
		}
	})

	t.Run("duplicate AC id", func(t *testing.T) {
		root := fixture{
			Features: []featureDef{{Name: "AUTH", Reqs: []reqDef{
				{ID: "AUTH-1", ACs: []acDef{{ID: "AUTH-1-1"}}},
			}}},
		}.build(t)
		writeReq(t, root, "auth_dup_ac.req", featureDef{Name: "AUTH", Reqs: []reqDef{
			{ID: "AUTH-2", ACs: []acDef{{ID: "AUTH-1-1"}}},
		}})
		proj := mustLoad(t, root)
		if !anyError(proj.ValidationErrors(), "duplicate", "AUTH-1-1") {
			t.Errorf("expected duplicate AUTH-1-1 error in %+v", proj.ValidationErrors())
		}
	})

	t.Run("prefix mismatch", func(t *testing.T) {
		root := fixture{
			Features: []featureDef{{Name: "AUTH", Reqs: []reqDef{
				{ID: "BILL-1", ACs: []acDef{{ID: "BILL-1-1"}}},
			}}},
		}.build(t)
		proj := mustLoad(t, root)
		if !anyError(proj.ValidationErrors(), "BILL-1") {
			t.Errorf("expected prefix mismatch error in %+v", proj.ValidationErrors())
		}
	})
}

func anyError(errs []api.ValidationError, substrs ...string) bool {
	for _, e := range errs {
		all := true
		for _, s := range substrs {
			if !strings.Contains(e.Message, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// ----- API-4: Report Formatting -----

func formatFixtureProject(t *testing.T) (*api.Project, *spool.Report) {
	t.Helper()
	root := fixture{
		Features: []featureDef{{Name: "AUTH", Reqs: []reqDef{
			{ID: "AUTH-1", Title: "Login", Status: "active", ACs: []acDef{
				{ID: "AUTH-1-1", Title: "Valid creds"},
			}},
		}}},
		Tests: []testFileDef{
			{Path: "auth_test.go", Body: goTest("TestLogin", "AUTH-1-1")},
		},
	}.build(t)
	proj := mustLoad(t, root)
	return proj, proj.Report()
}

// API-4-1
func TestFormatReport_JSON(t *testing.T) {
	_, r := formatFixtureProject(t)
	got, err := api.FormatReport(r, "json")
	if err != nil {
		t.Fatalf("FormatReport(json): %v", err)
	}
	want, err := reporter.JSON{}.Format(r)
	if err != nil {
		t.Fatalf("reporter.JSON: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("FormatReport(json) does not match reporter.JSON output")
	}
	// Sanity: result is valid JSON.
	var sink any
	if err := json.Unmarshal(got, &sink); err != nil {
		t.Errorf("FormatReport(json) is not valid JSON: %v", err)
	}
}

// API-4-2
func TestFormatReport_Text(t *testing.T) {
	_, r := formatFixtureProject(t)
	got, err := api.FormatReport(r, "text")
	if err != nil {
		t.Fatalf("FormatReport(text): %v", err)
	}
	want, err := reporter.Text{}.Format(r)
	if err != nil {
		t.Fatalf("reporter.Text: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("FormatReport(text) does not match reporter.Text output")
	}
}

// API-4-3
func TestFormatReport_Markdown(t *testing.T) {
	_, r := formatFixtureProject(t)
	got, err := api.FormatReport(r, "markdown")
	if err != nil {
		t.Fatalf("FormatReport(markdown): %v", err)
	}
	want, err := reporter.Markdown{}.Format(r)
	if err != nil {
		t.Fatalf("reporter.Markdown: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("FormatReport(markdown) does not match reporter.Markdown output")
	}
}

// API-4-4
func TestFormatReport_UnknownFormatRejected(t *testing.T) {
	_, r := formatFixtureProject(t)
	_, err := api.FormatReport(r, "yaml")
	if err == nil {
		t.Fatalf("FormatReport(yaml) returned nil error, want non-nil")
	}
	msg := strings.ToLower(err.Error())
	for _, want := range []string{"json", "text", "markdown"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message does not list %q: %q", want, err.Error())
		}
	}
}
