// Package index builds a queryable traceability graph from parser and scanner output.
package index

import (
	"fmt"
	"sort"
	"strings"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/scanner"
)

// Index is the traceability graph joining requirements with test annotations.
type Index struct {
	features     map[string]*Feature
	requirements map[string]*IndexedReq
	acs          map[string]*IndexedAC
	errors       []Error
	orphaned     []OrphanedAnnotation
}

// Feature groups requirements under a declared feature name.
type Feature struct {
	Name         string
	Requirements []*IndexedReq
}

// IndexedReq is a requirement enriched with index metadata.
type IndexedReq struct {
	parser.Requirement
	Feature    string
	SourceFile string
	ACs        []*IndexedAC
}

// IndexedAC is an acceptance criterion enriched with linked tests.
type IndexedAC struct {
	parser.AcceptanceCriterion
	ReqID      string
	Feature    string
	SourceFile string
	Tests      []LinkedTest
}

// LinkedTest associates a test function with the file it was found in.
type LinkedTest struct {
	Function spool.TestFunction
	File     string
}

// OrphanedAnnotation is a test annotation referencing an ID not in any req file.
type OrphanedAnnotation struct {
	ID   string
	File string
	Line int
}

// Error is a validation error found during index construction.
type Error struct {
	Message string
}

// Build constructs an index from parsed req files and scan results.
func Build(reqFiles []*parser.ReqFile, scanResults []scanner.FileResult) *Index {
	idx := &Index{
		features:     make(map[string]*Feature),
		requirements: make(map[string]*IndexedReq),
		acs:          make(map[string]*IndexedAC),
	}

	// Phase 1: Populate from req files.
	for _, rf := range reqFiles {
		feat, ok := idx.features[rf.Feature]
		if !ok {
			feat = &Feature{Name: rf.Feature}
			idx.features[rf.Feature] = feat
		}

		for _, r := range rf.Requirements {
			// Validate feature prefix.
			prefix := idPrefix(r.ID)
			if !strings.EqualFold(prefix, rf.Feature) {
				idx.errors = append(idx.errors, Error{
					Message: fmt.Sprintf("requirement %s has prefix %q but is in feature %q (%s)", r.ID, prefix, rf.Feature, rf.SourceFile),
				})
			}

			// Check uniqueness.
			if existing, dup := idx.requirements[r.ID]; dup {
				idx.errors = append(idx.errors, Error{
					Message: fmt.Sprintf("duplicate requirement ID %s in %s and %s", r.ID, existing.SourceFile, rf.SourceFile),
				})
				continue
			}

			ir := &IndexedReq{
				Requirement: r,
				Feature:     rf.Feature,
				SourceFile:  rf.SourceFile,
			}

			for _, a := range r.AcceptanceCriteria {
				if existing, dup := idx.acs[a.ID]; dup {
					idx.errors = append(idx.errors, Error{
						Message: fmt.Sprintf("duplicate AC ID %s in %s and %s", a.ID, existing.SourceFile, rf.SourceFile),
					})
					continue
				}

				ia := &IndexedAC{
					AcceptanceCriterion: a,
					ReqID:              r.ID,
					Feature:            rf.Feature,
					SourceFile:         rf.SourceFile,
				}
				idx.acs[a.ID] = ia
				ir.ACs = append(ir.ACs, ia)
			}

			idx.requirements[r.ID] = ir
			feat.Requirements = append(feat.Requirements, ir)
		}
	}

	// Phase 2: Link test annotations and detect orphans.
	for _, fr := range scanResults {
		for _, m := range fr.Mappings {
			for _, ann := range m.Annotations {
				if ia, ok := idx.acs[ann.ID]; ok {
					ia.Tests = append(ia.Tests, LinkedTest{
						Function: m.Function,
						File:     fr.Path,
					})
				} else {
					idx.orphaned = append(idx.orphaned, OrphanedAnnotation{
						ID:   ann.ID,
						File: fr.Path,
						Line: ann.Line,
					})
				}
			}
		}
	}

	return idx
}

// Features returns all features in the index.
func (idx *Index) Features() []Feature {
	out := make([]Feature, 0, len(idx.features))
	for _, f := range idx.features {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Feature returns the feature with the given name, or nil.
func (idx *Index) Feature(name string) *Feature {
	return idx.features[name]
}

// Requirement returns the requirement with the given ID, or nil.
func (idx *Index) Requirement(id string) *IndexedReq {
	return idx.requirements[id]
}

// AC returns the acceptance criterion with the given ID, or nil.
func (idx *Index) AC(id string) *IndexedAC {
	return idx.acs[id]
}

// UntestedACs returns all ACs belonging to active requirements with no linked tests.
func (idx *Index) UntestedACs() []*IndexedAC {
	var out []*IndexedAC
	for _, ir := range idx.requirements {
		if ir.Status != "active" {
			continue
		}
		for _, ia := range ir.ACs {
			if len(ia.Tests) == 0 {
				out = append(out, ia)
			}
		}
	}
	return out
}

// OrphanedAnnotations returns annotations referencing IDs not in any req file.
func (idx *Index) OrphanedAnnotations() []OrphanedAnnotation {
	return idx.orphaned
}

// Errors returns validation errors found during index construction.
func (idx *Index) Errors() []Error {
	return idx.errors
}

// idPrefix extracts the prefix from a Spool ID (e.g., "AUTH" from "AUTH-1-1").
func idPrefix(id string) string {
	if i := strings.Index(id, "-"); i > 0 {
		return id[:i]
	}
	return id
}
