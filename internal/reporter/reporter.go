// Package reporter generates traceability reports from the index.
package reporter

import (
	"io"
	"os"
	"sort"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/index"
)

// Generate produces a Report from the index.
func Generate(idx *index.Index) *spool.Report {
	r := &spool.Report{}

	for _, feat := range idx.Features() {
		fr := spool.FeatureReport{Name: feat.Name}

		for _, req := range feat.Requirements {
			if req.Status != "active" {
				fr.ExcludedRequirements++
				continue
			}
			fr.Requirements++
			for _, ac := range req.ACs {
				fr.ACs++
				if len(ac.Tests) > 0 {
					fr.LinkedACs++
				} else {
					fr.UntestedACs++
				}
			}
		}

		r.TotalRequirements += fr.Requirements
		r.TotalACs += fr.ACs
		r.LinkedACs += fr.LinkedACs
		r.UntestedACs += fr.UntestedACs
		r.Features = append(r.Features, fr)
	}

	for _, ua := range idx.UntestedACs() {
		r.Untested = append(r.Untested, spool.UntestedAC{
			ID:          ua.ID,
			Description: ua.Description,
			Feature:     ua.Feature,
		})
	}

	for _, o := range idx.OrphanedAnnotations() {
		r.Orphaned = append(r.Orphaned, spool.OrphanedRef{
			ID:   o.ID,
			File: o.File,
			Line: o.Line,
		})
	}
	r.OrphanedCount = len(r.Orphaned)

	sort.Slice(r.Untested, func(i, j int) bool { return r.Untested[i].ID < r.Untested[j].ID })

	return r
}

// Write writes data to a file at dest. If dest is empty, writes to stdout.
func Write(data []byte, dest string) error {
	if dest == "" {
		return WriteTo(data, os.Stdout)
	}
	return os.WriteFile(dest, data, 0644)
}

// WriteTo writes data to the given writer.
func WriteTo(data []byte, w io.Writer) error {
	_, err := w.Write(data)
	return err
}
