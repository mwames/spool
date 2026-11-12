package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// orderFirstReq is the ORDA feature, which the report lists before ORDB.
// Its requirements are written out of ID order, and each has a missing
// interface path and an AC not written as Given/When/Then. Between the
// active ORDA-2, ORDA-1 and ORDA-3 are two drafts the report doesn't count
// (SPEC-4-6): ORDA-9, written draft, with two missing paths and two such ACs,
// each out of order, and ORDA-5, written active with only a draft AC and so
// draft (SPEC-4-4).
const orderFirstReq = `feature: ORDA

requirements:

  - id: ORDA-2
    title: Two
    description: Active.
    status: active
    interface: a/two.go
    acceptance_criteria:
      - id: ORDA-2-1
        title: Two Unformed
        description: Spool reports it.

  - id: ORDA-9
    title: Nine
    description: A draft.
    status: draft
    interface:
      - a/nine-z.go
      - a/nine-a.go
    acceptance_criteria:
      - id: ORDA-9-2
        title: Nine Second Unformed
        description: Spool reports it.
      - id: ORDA-9-1
        title: Nine First Unformed
        description: Spool reports it.

  - id: ORDA-1
    title: One
    description: Active.
    status: active
    interface: a/one.go
    acceptance_criteria:
      - id: ORDA-1-1
        title: One Unformed
        description: Spool reports it.

  - id: ORDA-5
    title: Five
    description: Written active, with only a draft AC.
    status: active
    interface: a/five.go
    acceptance_criteria:
      - id: ORDA-5-1
        title: Five Unformed
        description: Spool reports it.
        status: draft

  - id: ORDA-3
    title: Three
    description: Active.
    status: active
    interface: a/three.go
    acceptance_criteria:
      - id: ORDA-3-1
        title: Three Unformed
        description: Spool reports it.
`

// orderLastReq is the ORDB feature, in a file read before orderFirstReq's,
// with one active requirement that has a missing interface path and an AC
// not written as Given/When/Then.
const orderLastReq = `feature: ORDB

requirements:

  - id: ORDB-1
    title: Other
    description: Active.
    status: active
    interface: b/one.go
    acceptance_criteria:
      - id: ORDB-1-1
        title: Other Unformed
        description: Spool reports it.
`

// orderProject writes orderFirstReq and orderLastReq with a test for each
// active AC. None of their interface paths exist.
func orderProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".spool.yaml":       lintConfig("", ""),
		".spool/z_orda.req": orderFirstReq,
		".spool/a_ordb.req": orderLastReq,
		"pkg/order_test.go": annotatedTest("ORDA-2-1", "ORDA-1-1", "ORDA-3-1", "ORDB-1-1"),
	})
	return root
}

// orderMissing is orderProject's missing interfaces, in order: as the
// report's features, then as the requirements are in the .req file, counted
// or not, then as the paths are written.
var orderMissing = []idPath{
	{"ORDA-2", "a/two.go"},
	{"ORDA-9", "a/nine-z.go"},
	{"ORDA-9", "a/nine-a.go"},
	{"ORDA-1", "a/one.go"},
	{"ORDA-5", "a/five.go"},
	{"ORDA-3", "a/three.go"},
	{"ORDB-1", "b/one.go"},
}

// orderNotGWT is orderProject's ACs not written as Given/When/Then, with
// their titles, in order: as the report's features, then as the requirements
// are in the .req file, counted or not, then as the ACs are written.
var orderNotGWT = []struct{ ID, Feature, Title string }{
	{"ORDA-2-1", "ORDA", "Two Unformed"},
	{"ORDA-9-2", "ORDA", "Nine Second Unformed"},
	{"ORDA-9-1", "ORDA", "Nine First Unformed"},
	{"ORDA-1-1", "ORDA", "One Unformed"},
	{"ORDA-5-1", "ORDA", "Five Unformed"},
	{"ORDA-3-1", "ORDA", "Three Unformed"},
	{"ORDB-1-1", "ORDB", "Other Unformed"},
}

// checkOrderFixture fails the test unless the report lists ORDA then ORDB
// and counts ORDA-2, ORDA-1, ORDA-3 and ORDB-1 only, so that the drafts
// with findings sit between counted requirements in a feature that isn't
// last.
func checkOrderFixture(t *testing.T, root string) {
	t.Helper()
	_, stdout, _ := run(t, root, "--format", "json")
	var report struct {
		Features []struct {
			Name string `json:"name"`
		} `json:"features"`
		Requirements []struct {
			ID string `json:"id"`
		} `json:"requirements"`
	}
	decodeOne(t, stdout, &report)
	var features, reqs []string
	for _, f := range report.Features {
		features = append(features, f.Name)
	}
	for _, r := range report.Requirements {
		reqs = append(reqs, r.ID)
	}
	if got := strings.Join(features, ","); got != "ORDA,ORDB" {
		t.Errorf("fixture: features = %s, want ORDA,ORDB", got)
	}
	if got := strings.Join(reqs, ","); got != "ORDA-2,ORDA-1,ORDA-3,ORDB-1" {
		t.Errorf("fixture: requirements = %s, want ORDA-2,ORDA-1,ORDA-3,ORDB-1", got)
	}
}

// markdownInOrder checks that, for each want in turn, some line of section
// holds every string of it, each found on a later line than the one before.
func markdownInOrder(t *testing.T, out string, section []string, want [][]string) {
	t.Helper()
	last := -1
	for _, w := range want {
		at := -1
		for i, l := range section {
			all := true
			for _, s := range w {
				if !strings.Contains(l, s) {
					all = false
					break
				}
			}
			if all {
				at = i
				break
			}
		}
		if at < 0 {
			t.Errorf("no line gives %q:\n%s", w, out)
			continue
		}
		if at <= last {
			t.Errorf("%q is listed before the finding it follows, want them in order %q:\n%s", w, want, out)
		}
		last = at
	}
}

// SPEC-1-3
func TestRun_MissingInterfacesOrderText(t *testing.T) {
	root := orderProject(t)
	checkOrderFixture(t, root)
	_, stdout, stderr := run(t, root)
	got, ok := textSection(stdout, "Missing Interfaces")
	if !ok {
		t.Fatalf("no \"Missing Interfaces\" heading in:\n%s\nstderr:\n%s", stdout, stderr)
	}
	var want []string
	for _, f := range orderMissing {
		want = append(want, f.ID+": "+f.Path)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Missing Interfaces entries = %q\nwant %q\n%s", got, want, stdout)
	}
}

// SPEC-1-3
func TestRun_MissingInterfacesOrderMarkdown(t *testing.T) {
	root := orderProject(t)
	checkOrderFixture(t, root)
	_, stdout, _ := run(t, root, "--format", "markdown")
	section, ok := markdownSection(stdout, "Missing Interfaces")
	if !ok {
		t.Fatalf("no \"Missing Interfaces\" heading in:\n%s", stdout)
	}
	var want [][]string
	for _, f := range orderMissing {
		want = append(want, []string{f.ID, f.Path})
	}
	markdownInOrder(t, stdout, section, want)
}

// SPEC-1-3
func TestRun_MissingInterfacesOrderJSON(t *testing.T) {
	root := orderProject(t)
	checkOrderFixture(t, root)
	_, stdout, _ := run(t, root, "--format", "json")
	var got []idPath
	for _, raw := range jsonList(t, stdout, "missingInterfaces", []string{"id", "path"}) {
		var f idPath
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		got = append(got, f)
	}
	if !reflect.DeepEqual(got, orderMissing) {
		t.Errorf("missingInterfaces = %+v\nwant %+v", got, orderMissing)
	}
}

// SPEC-3-2
func TestRun_NotGivenWhenThenOrderText(t *testing.T) {
	root := orderProject(t)
	checkOrderFixture(t, root)
	_, stdout, stderr := run(t, root)
	got, ok := textSection(stdout, "Not Given/When/Then")
	if !ok {
		t.Fatalf("no \"Not Given/When/Then\" heading in:\n%s\nstderr:\n%s", stdout, stderr)
	}
	var want []string
	for _, f := range orderNotGWT {
		want = append(want, f.ID+": "+f.Title)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Not Given/When/Then entries = %q\nwant %q\n%s", got, want, stdout)
	}
}

// SPEC-3-2
func TestRun_NotGivenWhenThenOrderMarkdown(t *testing.T) {
	root := orderProject(t)
	checkOrderFixture(t, root)
	_, stdout, _ := run(t, root, "--format", "markdown")
	section, ok := markdownSection(stdout, "Not Given/When/Then")
	if !ok {
		t.Fatalf("no \"Not Given/When/Then\" heading in:\n%s", stdout)
	}
	var want [][]string
	for _, f := range orderNotGWT {
		want = append(want, []string{f.ID})
	}
	markdownInOrder(t, stdout, section, want)
}

// SPEC-3-2
func TestRun_NotGivenWhenThenOrderJSON(t *testing.T) {
	root := orderProject(t)
	checkOrderFixture(t, root)
	_, stdout, _ := run(t, root, "--format", "json")
	var got []idFeature
	for _, raw := range jsonList(t, stdout, "notGivenWhenThen", []string{"feature", "id"}) {
		var f idFeature
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		got = append(got, f)
	}
	var want []idFeature
	for _, f := range orderNotGWT {
		want = append(want, idFeature{f.ID, f.Feature})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("notGivenWhenThen = %+v\nwant %+v", got, want)
	}
}
