package lsp

import (
	"testing"

	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

func TestBuildTraceabilityTree_WithCoverage(t *testing.T) {
	idx := buildTestIndex(
		[]*parser.ReqFile{reqFile("AUTH", req("AUTH-1", "active",
			ac("AUTH-1-1", 10),
			ac("AUTH-1-2", 15),
		))},
		[]scanner.FileResult{scanResult("/tests/auth_test.go", mapping("TestLogin", 5, "AUTH-1-1"))},
	)

	tree := buildTraceabilityTree(idx)
	if len(tree.Features) != 1 {
		t.Fatalf("len(features) = %d, want 1", len(tree.Features))
	}

	f := tree.Features[0]
	if f.Name != "AUTH" {
		t.Errorf("feature name = %q, want AUTH", f.Name)
	}
	if len(f.Requirements) != 1 {
		t.Fatalf("len(reqs) = %d, want 1", len(f.Requirements))
	}

	r := f.Requirements[0]
	if r.ID != "AUTH-1" {
		t.Errorf("req id = %q, want AUTH-1", r.ID)
	}
	if len(r.ACs) != 2 {
		t.Fatalf("len(acs) = %d, want 2", len(r.ACs))
	}

	if r.ACs[0].ID != "AUTH-1-1" || r.ACs[0].TestCount != 1 {
		t.Errorf("AC[0] = {%s, %d tests}, want {AUTH-1-1, 1}", r.ACs[0].ID, r.ACs[0].TestCount)
	}
	if r.ACs[1].ID != "AUTH-1-2" || r.ACs[1].TestCount != 0 {
		t.Errorf("AC[1] = {%s, %d tests}, want {AUTH-1-2, 0}", r.ACs[1].ID, r.ACs[1].TestCount)
	}
}

func TestBuildTraceabilityTree_EmptyIndex(t *testing.T) {
	idx := buildTestIndex(nil, nil)

	tree := buildTraceabilityTree(idx)
	if len(tree.Features) != 0 {
		t.Errorf("len(features) = %d, want 0", len(tree.Features))
	}
}
