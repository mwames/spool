package reporter

import (
	"fmt"
	"strings"

	spool "github.com/mwames/Spool"
)

// Text formats a report as plain text suitable for terminal output.
type Text struct{}

func (Text) Format(r *spool.Report) ([]byte, error) {
	var b strings.Builder

	b.WriteString("Spool Traceability Report\n")
	b.WriteString("========================\n\n")

	// Summary table
	if len(r.Features) > 0 {
		b.WriteString(fmt.Sprintf("%-20s %6s %6s %8s %10s %10s\n", "Feature", "Reqs", "ACs", "Linked", "Untested", "Excluded"))
		b.WriteString(fmt.Sprintf("%-20s %6s %6s %8s %10s %10s\n", "-------", "----", "---", "------", "--------", "--------"))
		for _, f := range r.Features {
			b.WriteString(fmt.Sprintf("%-20s %6d %6d %8d %10d %10d\n", f.Name, f.Requirements, f.ACs, f.LinkedACs, f.UntestedACs, f.ExcludedRequirements))
		}
		b.WriteString(fmt.Sprintf("\nTotals: %d requirements, %d ACs, %d linked, %d untested, %d orphaned\n", r.TotalRequirements, r.TotalACs, r.LinkedACs, r.UntestedACs, r.OrphanedCount))
	}

	// Untested ACs
	if len(r.Untested) > 0 {
		b.WriteString("\nUntested Acceptance Criteria\n")
		b.WriteString("----------------------------\n")
		for _, u := range r.Untested {
			b.WriteString(fmt.Sprintf("  %s: %s\n", u.ID, u.Description))
		}
	}

	// Orphaned annotations
	if len(r.Orphaned) > 0 {
		b.WriteString("\nOrphaned Annotations\n")
		b.WriteString("--------------------\n")
		for _, o := range r.Orphaned {
			b.WriteString(fmt.Sprintf("  %s at %s:%d\n", o.ID, o.File, o.Line))
		}
	}

	// No issues
	if len(r.Untested) == 0 && len(r.Orphaned) == 0 && r.TotalACs > 0 {
		b.WriteString("\nFull traceability — no issues found.\n")
	}

	return []byte(b.String()), nil
}
