package lsp

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/uri"
)

func acStatus(id, status string, line int) parser.AcceptanceCriterion {
	a := ac(id, line)
	a.Status = status
	return a
}

func invStatus(id, status string, line int) parser.Invariant {
	v := inv(id, line)
	v.Status = status
	return v
}

// LSP-5-1
// INVARIANT-2-3
// SPEC-4-5
func TestBuildDiagnostics_UntestedOnlyForActiveItems(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH",
			// Written active, folded draft.
			withInvariants(req("AUTH-1", "draft",
				acStatus("AUTH-1-1", "active", 6),
				acStatus("AUTH-1-2", "draft", 8),
				acStatus("AUTH-1-3", "deprecated", 10),
			),
				invStatus("AUTH-1-INV-1", "active", 13),
				invStatus("AUTH-1-INV-2", "draft", 15),
				invStatus("AUTH-1-INV-3", "deprecated", 17),
			),
			// Written deprecated, folded active.
			withInvariants(req("AUTH-2", "active",
				acStatus("AUTH-2-1", "active", 22),
				acStatus("AUTH-2-2", "deprecated", 24),
			),
				invStatus("AUTH-2-INV-1", "deprecated", 27),
			),
			withInvariants(req("AUTH-3", "superseded",
				acStatus("AUTH-3-1", "active", 32),
			),
				invStatus("AUTH-3-INV-1", "active", 35),
			),
		)},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestTwo", 5, "AUTH-2-1"))},
	)

	fileDiags := buildDiagnostics(idx)[uri.File("/reqs/AUTH.req")]
	var got []string
	for _, d := range fileDiags {
		got = append(got, d.Message)
	}
	want := map[uint32]string{5: "AUTH-1-1", 12: "AUTH-1-INV-1"} // 0-based lines
	if len(fileDiags) != len(want) {
		t.Fatalf("diagnostics = %q, want one each for the untested active AUTH-1-1 and AUTH-1-INV-1", got)
	}
	for _, d := range fileDiags {
		id, ok := want[d.Range.Start.Line]
		if !ok || !strings.Contains(d.Message, id) {
			t.Errorf("diagnostic %q on line %d, want the untested warnings %v", d.Message, d.Range.Start.Line, want)
		}
	}
}

// SPEC-4-7
func TestBuildTraceabilityTree_ItemStatus(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH",
			withInvariants(req("AUTH-1", "draft",
				acStatus("AUTH-1-1", "active", 6),
				acStatus("AUTH-1-2", "draft", 8),
				acStatus("AUTH-1-3", "deprecated", 10),
			),
				invStatus("AUTH-1-INV-1", "deprecated", 13),
				invStatus("AUTH-1-INV-2", "draft", 15),
			),
			withInvariants(req("AUTH-2", "superseded", acStatus("AUTH-2-1", "superseded", 20)),
				invStatus("AUTH-2-INV-1", "active", 23),
			),
		)},
		nil,
	)

	tree := buildTraceabilityTree(idx)
	got := map[string]string{}
	for _, f := range tree.Features {
		for _, r := range f.Requirements {
			got[r.ID] = r.Status
			for _, a := range r.ACs {
				got[a.ID] = a.Status
			}
			for _, v := range r.Invariants {
				got[v.ID] = v.Status
			}
		}
	}
	want := map[string]string{
		"AUTH-1":       "draft",
		"AUTH-1-1":     "active",
		"AUTH-1-2":     "draft",
		"AUTH-1-3":     "deprecated",
		"AUTH-1-INV-1": "deprecated",
		"AUTH-1-INV-2": "draft",
		"AUTH-2":       "superseded",
		"AUTH-2-1":     "superseded",
		"AUTH-2-INV-1": "active",
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s status = %q, want %q", id, got[id], w)
		}
	}
}

// itemStatusReq is a .req file whose requirements are written with one
// status and folded to another (SPEC-4-4).
const itemStatusReq = `feature: ITEM

requirements:
  - id: ITEM-1
    title: Folded Draft
    description: Written active, with a draft AC.
    status: active
    acceptance_criteria:
      - id: ITEM-1-1
        title: Inherited
        description: Given a, when b, then c.
      - id: ITEM-1-2
        title: Draft
        description: Given a, when b, then c.
        status: draft
    invariants:
      - id: ITEM-1-INV-1
        title: Deprecated
        description: It holds.
        status: deprecated
  - id: ITEM-2
    title: Folded Active
    description: Written deprecated, with an active AC.
    status: deprecated
    acceptance_criteria:
      - id: ITEM-2-1
        title: Active
        description: Given a, when b, then c.
        status: active
      - id: ITEM-2-2
        title: Inherited
        description: Given a, when b, then c.
    invariants:
      - id: ITEM-2-INV-1
        title: Inherited
        description: It holds.
`

// SPEC-4-4
// SPEC-4-7
func TestHandleTraceability_ItemStatusFromDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".spool.yaml"), "test_patterns:\n  - \"**/*_test.go\"\n")
	writeFile(t, filepath.Join(dir, ".spool", "item.req"), itemStatusReq)
	srv := NewServer("")
	initialize(t, srv, dir)

	msg, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(2), "spool/traceability", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := call(t, srv, msg)

	type item struct {
		ID     string  `json:"id"`
		Status *string `json:"status"`
	}
	var tree struct {
		Features []struct {
			Requirements []struct {
				item
				ACs        []item `json:"acs"`
				Invariants []item `json:"invariants"`
			} `json:"requirements"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	got := map[string]string{}
	for _, f := range tree.Features {
		for _, r := range f.Requirements {
			for _, it := range append(append([]item{r.item}, r.ACs...), r.Invariants...) {
				if it.Status == nil {
					t.Errorf("%s has no \"status\" in %s", it.ID, raw)
					continue
				}
				got[it.ID] = *it.Status
			}
		}
	}
	want := map[string]string{
		"ITEM-1":       "draft",
		"ITEM-1-1":     "active",
		"ITEM-1-2":     "draft",
		"ITEM-1-INV-1": "deprecated",
		"ITEM-2":       "active",
		"ITEM-2-1":     "active",
		"ITEM-2-2":     "deprecated",
		"ITEM-2-INV-1": "deprecated",
	}
	var ids []string
	for id := range want {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if got[id] != want[id] {
			t.Errorf("%s status = %q, want %q", id, got[id], want[id])
		}
	}
}

// SPEC-4-5
func TestBuildDiagnostics_ItemStatusFromDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".spool.yaml"), "test_patterns:\n  - \"**/*_test.go\"\n")
	writeFile(t, filepath.Join(dir, ".spool", "item.req"), itemStatusReq)
	srv := NewServer("")
	initialize(t, srv, dir)

	var got []string
	for _, ds := range buildDiagnostics(srv.idx) {
		for _, d := range ds {
			got = append(got, d.Message)
		}
	}
	// ITEM-1-1 and ITEM-2-1 are the active items, both untested.
	if len(got) != 2 {
		t.Fatalf("diagnostics = %q, want untested warnings for ITEM-1-1 and ITEM-2-1 only", got)
	}
	for _, id := range []string{"ITEM-1-1", "ITEM-2-1"} {
		found := false
		for _, m := range got {
			found = found || strings.Contains(m, id)
		}
		if !found {
			t.Errorf("diagnostics = %q, want one naming %s", got, id)
		}
	}
}
