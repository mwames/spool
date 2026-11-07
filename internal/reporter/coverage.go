package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/index"
)

// Coverage reports the given normalized AC and invariant IDs, in order, with
// their linked tests. Test files are made relative to root (COVERAGE-2).
func Coverage(idx *index.Index, root string, ids []string) *spool.CoverageReport {
	r := &spool.CoverageReport{Total: len(ids)}
	for _, id := range ids {
		e := spool.CoverageEntry{ID: id, Kind: "unknown", Tests: []spool.CoverageTest{}}
		var linked []index.LinkedTest
		if ac := idx.AC(id); ac != nil {
			e.Kind, linked = "ac", ac.Tests
		} else if inv := idx.Invariant(id); inv != nil {
			e.Kind, linked = "invariant", inv.Tests
		}
		seen := map[spool.CoverageTest]bool{}
		for _, lt := range linked {
			file := lt.File
			if rel, err := filepath.Rel(root, file); err == nil {
				file = rel
			}
			t := spool.CoverageTest{File: filepath.ToSlash(file), Line: lt.Function.Line, Function: lt.Function.Name}
			if !seen[t] {
				seen[t] = true
				e.Tests = append(e.Tests, t)
			}
		}
		sort.SliceStable(e.Tests, func(i, j int) bool {
			a, b := e.Tests[i], e.Tests[j]
			if a.File != b.File {
				return a.File < b.File
			}
			return a.Line < b.Line
		})
		e.Covered = len(e.Tests) > 0
		if e.Covered {
			r.Covered++
		}
		r.IDs = append(r.IDs, e)
	}
	return r
}

func entryState(e spool.CoverageEntry) string {
	switch {
	case e.Kind == "unknown":
		return "unknown"
	case e.Covered:
		return "covered"
	}
	return "untested"
}

// CoverageText formats a coverage report as text: each ID and its state on
// one line, its tests indented below as "<file>:<line> <function>", and a
// last line "<n>/<m> covered" (COVERAGE-2-1).
func CoverageText(r *spool.CoverageReport) []byte {
	var buf bytes.Buffer
	for _, e := range r.IDs {
		fmt.Fprintf(&buf, "%s %s\n", e.ID, entryState(e))
		for _, t := range e.Tests {
			fmt.Fprintf(&buf, "  %s:%d %s\n", t.File, t.Line, t.Function)
		}
	}
	fmt.Fprintf(&buf, "%d/%d covered\n", r.Covered, r.Total)
	return buf.Bytes()
}

type jsonCoverageTest struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

type jsonCoverageEntry struct {
	ID      string             `json:"id"`
	Kind    string             `json:"kind"`
	Covered bool               `json:"covered"`
	Tests   []jsonCoverageTest `json:"tests"`
}

type jsonCoverage struct {
	IDs     []jsonCoverageEntry `json:"ids"`
	Covered int                 `json:"covered"`
	Total   int                 `json:"total"`
}

// CoverageJSON formats a coverage report as JSON (COVERAGE-2-6).
func CoverageJSON(r *spool.CoverageReport) ([]byte, error) {
	out := jsonCoverage{IDs: []jsonCoverageEntry{}, Covered: r.Covered, Total: r.Total}
	for _, e := range r.IDs {
		je := jsonCoverageEntry{ID: e.ID, Kind: e.Kind, Covered: e.Covered, Tests: []jsonCoverageTest{}}
		for _, t := range e.Tests {
			je.Tests = append(je.Tests, jsonCoverageTest{File: t.File, Line: t.Line, Function: t.Function})
		}
		out.IDs = append(out.IDs, je)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// CoverageExitCode returns 0 when every listed ID is covered and 1 otherwise,
// whatever the configured severities (COVERAGE-2-2, COVERAGE-2-3).
func CoverageExitCode(r *spool.CoverageReport) int {
	if r.Covered == r.Total {
		return 0
	}
	return 1
}
